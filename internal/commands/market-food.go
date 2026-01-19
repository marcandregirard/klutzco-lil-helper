package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Food items (name_id) and their healing values
var foodHealingValues = map[string]int{
	"cooked_piranha":       2,
	"cooked_perch":         3,
	"cooked_mackerel":      4,
	"cooked_cod":           6,
	"cooked_trout":         7,
	"cooked_salmon":        8,
	"cooked_carp":          10,
	"cooked_zander":        12,
	"cooked_pufferfish":    14,
	"cooked_anglerfish":    16,
	"cooked_tuna":          17,
	"cooked_bloodmoon_eel": 24,
	"cooked_meat":          4,
	"cooked_giant_meat":    8,
	"cooked_quality_meat":  12,
	"cooked_superior_meat": 18,
	"cooked_apex_meat":     20,
	"potato_soup":          5,
	"meat_burger":          7,
	"cod_soup":             10,
	"blueberry_pie":        11,
	"salmon_salad":         14,
	"porcini_soup":         17,
	"stew":                 19,
	"power_pizza":          22,
}

// Item ID mapping (name_id -> internal_id)
// This is static data that never changes
var itemIDMapping = map[string]int{
	"cooked_mackerel":      100,
	"cooked_perch":         102,
	"cooked_trout":         104,
	"cooked_salmon":        105,
	"cooked_carp":          106,
	"cooked_meat":          114,
	"cooked_giant_meat":    115,
	"cooked_quality_meat":  116,
	"cooked_superior_meat": 117,
	"potato_soup":          140,
	"meat_burger":          141,
	"cod_soup":             143,
	"blueberry_pie":        144,
	"salmon_salad":         145,
	"porcini_soup":         146,
	"power_pizza":          148,
	"cooked_anglerfish":    156,
	"cooked_zander":        158,
	"cooked_piranha":       160,
	"cooked_pufferfish":    162,
	"cooked_cod":           164,
	"stew":                 559,
	"cooked_tuna":          562,
	"cooked_bloodmoon_eel": 888,
	"cooked_apex_meat":     906,
}

// MarketPriceItem represents market price data
type MarketPriceItem struct {
	ItemID          int     `json:"itemId"`
	LowestSellPrice float64 `json:"lowestSellPrice"`
}

// FoodValueResult represents calculated results for display
type FoodValueResult struct {
	Name           string
	Healing        int
	Price          float64
	CostPerHealing float64
}

var marketFoodCommand = &discordgo.ApplicationCommand{
	Name:        "market-food",
	Description: "Show cost-effective food items based on current market prices",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "just_for_me",
			Description: "Only show the results to me.",
			Required:    false,
		},
	},
}

func marketFoodHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name != "market-food" {
		return
	}

	data := i.ApplicationCommandData()

	justForMe := false
	if len(data.Options) > 0 {
		justForMe = data.Options[0].BoolValue()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Fetch market prices
	priceMap, err := fetchMarketPrices(ctx)
	if err != nil {
		log.Printf("[market-food] failed to fetch market prices: %v", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Failed to fetch market data. The API may be temporarily unavailable. Please try again later.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Calculate food values
	results := calculateFoodValues(priceMap)

	if len(results) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "⚠️ No food items found with valid market prices. Try again later.",
				Flags:   ephemeralFlag(justForMe),
			},
		})
		return
	}

	// Filter dominated items
	totalCount := len(results)
	results = filterDominatedItems(results)
	log.Printf("[market-food] showing %d non-dominated items (filtered from %d)", len(results), totalCount)

	// Create and send embed
	embed := formatFoodEmbed(results)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  ephemeralFlag(justForMe),
		},
	})
}

// fetchMarketPrices fetches latest market prices from the API
func fetchMarketPrices(ctx context.Context) (map[int]float64, error) {
	url := "https://query.idleclans.com/api/PlayerMarket/items/prices/latest?includeAveragePrice=true"
	var marketData []MarketPriceItem
	var lastErr error

	// Retry logic: 3 attempts with exponential backoff
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API returned status %d", resp.StatusCode)
			continue
		}

		err = json.Unmarshal(body, &marketData)
		if err != nil {
			lastErr = err
			continue
		}

		// Success - build price map
		priceMap := make(map[int]float64)
		for _, item := range marketData {
			priceMap[item.ItemID] = item.LowestSellPrice
		}
		return priceMap, nil
	}

	return nil, fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

// calculateFoodValues combines data and calculates cost per HP
func calculateFoodValues(priceMap map[int]float64) []FoodValueResult {
	var results []FoodValueResult

	for foodName, healing := range foodHealingValues {
		itemID, ok := itemIDMapping[foodName]
		if !ok {
			log.Printf("[market-food] no item ID found for: %s", foodName)
			continue
		}

		price, ok := priceMap[itemID]
		if !ok {
			log.Printf("[market-food] no price data for: %s", foodName)
			continue
		}

		if price <= 0 {
			log.Printf("[market-food] invalid price for %s: %.2f", foodName, price)
			continue
		}

		costPerHealing := price / float64(healing)

		results = append(results, FoodValueResult{
			Name:           foodName,
			Healing:        healing,
			Price:          price,
			CostPerHealing: costPerHealing,
		})
	}

	// Sort by cost per healing (best value first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CostPerHealing < results[j].CostPerHealing
	})

	return results
}

// filterDominatedItems removes items that are economically dominated
// An item is dominated if another item exists that heals >= HP and costs < per HP
func filterDominatedItems(results []FoodValueResult) []FoodValueResult {
	var nonDominated []FoodValueResult

	for i := range results {
		isDominated := false

		// Check if this item is dominated by any other item
		for j := range results {
			if i != j {
				// Item j dominates item i if:
				// - j heals >= i's healing, AND
				// - j costs less per HP than i
				if results[j].Healing >= results[i].Healing &&
					results[j].CostPerHealing < results[i].CostPerHealing {
					isDominated = true
					break
				}
			}
		}

		if !isDominated {
			nonDominated = append(nonDominated, results[i])
		}
	}

	return nonDominated
}

// formatFoodEmbed creates the Discord embed for food values
func formatFoodEmbed(results []FoodValueResult) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "Market Food Values - Cost Effective Options",
		Description: fmt.Sprintf("Showing %d economically viable food items based on current market prices", len(results)),
		Color:       0x00FF00, // Green
		Fields:      []*discordgo.MessageEmbedField{},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Data from Idle Clans market API",
		},
	}

	// Add a field for each food item
	for _, result := range results {
		// Format the food name (titleize and replace underscores)
		foodName := strings.ReplaceAll(result.Name, "_", " ")
		foodName = titleizer.String(foodName)

		// Format the field value with healing, price, and cost per HP
		fieldValue := fmt.Sprintf(
			"Healing: **%d HP** | Price: **%.2f gold** | Cost: **%.2f g/HP**",
			result.Healing,
			result.Price,
			result.CostPerHealing,
		)

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   foodName,
			Value:  fieldValue,
			Inline: true,
		})
	}

	return embed
}
