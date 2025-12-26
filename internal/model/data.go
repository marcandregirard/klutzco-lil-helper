package model

import "strings"

var AllBosses = []Boss{
	{
		Name:           "zeus",
		AttackStyle:    "Magic",
		AttackWeakness: "Archery",
		Wiki:           "Zeus",
		TrimColor:      0xFFD700, // gold
		Key:            "godly",
	},
	{
		Name:           "medusa",
		AttackStyle:    "Archery",
		AttackWeakness: "Slash",
		Wiki:           "Medusa",
		TrimColor:      0xD3D3D3, // light grey
		Key:            "stone",
	},
	{
		Name:           "hades",
		AttackStyle:    "Magic",
		AttackWeakness: "Stab",
		Wiki:           "Hades",
		TrimColor:      0x0000FF, // blue
		Key:            "underworld",
	},
	{
		Name:           "griffin",
		AttackStyle:    "Melee",
		AttackWeakness: "Crush",
		Wiki:           "Griffin",
		TrimColor:      0xB8860B, // dark gold
		Key:            "mountain",
	},
	{
		Name:           "devil",
		AttackStyle:    "Melee",
		AttackWeakness: "Pound",
		Wiki:           "Devil",
		TrimColor:      0xFF0000, // red
		Key:            "burning",
	},
	{
		Name:           "chimera",
		AttackStyle:    "Melee",
		AttackWeakness: "Magic",
		Wiki:           "Chimera",
		TrimColor:      0x00FF00, // green
		Key:            "mutated",
	},
	{
		Name:           "sobek",
		AttackStyle:    "Archery",
		AttackWeakness: "None",
		Wiki:           "Sobek",
		TrimColor:      0x00FF00, // green
		Key:            "ancient",
	},
	{
		Name:           "kronos",
		AttackStyle:    "Archery,Magic,Melee",
		AttackWeakness: "Differs(Archery,Magic,Melee)",
		Wiki:           "Kronos",
		TrimColor:      0x00FF00, // green
		Key:            "krono's book",
	},
	{
		Name:           "mesines",
		AttackStyle:    "Melee/Magic",
		AttackWeakness: "Archery",
		Wiki:           "Mesines",
		TrimColor:      0x00FF00, // green
		Key:            "otherworldly",
	},
}

var BossesInformation = func() map[string]Boss {
	m := make(map[string]Boss)
	for _, b := range AllBosses {
		m[strings.ToLower(b.Name)] = b
	}
	return m
}()

var KeysInformation = func() map[string]Boss {
	m := make(map[string]Boss)
	for _, b := range AllBosses {
		m[strings.ToLower(b.Key)] = b
	}
	return m
}()
