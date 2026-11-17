package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
)

// inventoryTools returns the inventory and equipment tools.
func inventoryTools() []Tool {
	return []Tool{
		{
			Name: "inventory_add",
			Description: "Add an item to a character's inventory. Use when gear is " +
				"found, bought or granted; sample-table names take their printed " +
				"statistics, other names become plain items." + factsNote,
			Schema: objSchema(map[string]any{
				"actor": strProp("Character receiving the item."),
				"item":  strProp("Item name (sample-table name or free text)."),
				"qty":   intProp("Quantity to add (0 means 1).", 0),
				"tags":  strArrProp("Classification tags, e.g. \"ammunition\", \"weapon\", \"clothing\"."),
			}, "actor", "item"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor string   `json:"actor"`
					Item  string   `json:"item"`
					Qty   int      `json:"qty"`
					Tags  []string `json:"tags"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if err := requireField("item", a.Item); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.InventoryAdd{
					Actor: a.Actor, Item: a.Item, Qty: a.Qty, Tags: a.Tags,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "inventory_remove",
			Description: "Remove quantity from a character's inventory — or the whole " +
				"stack. Use when gear is consumed, lost or spent; the carry total after " +
				"the removal comes back as facts." + factsNote,
			Schema: objSchema(map[string]any{
				"actor": strProp("Character losing the item."),
				"item":  strProp("Item ID (\"item-3\") or inventory name."),
				"qty":   intProp("Quantity to remove (0 removes 1; larger than carried is refused).", 0),
			}, "actor", "item"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor string `json:"actor"`
					Item  string `json:"item"`
					Qty   int    `json:"qty"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if err := requireField("item", a.Item); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.InventoryRemove{
					Actor: a.Actor, Item: a.Item, Qty: a.Qty,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "equip",
			Description: "Ready an inventory item: a named slot (\"main\", \"off\") wields " +
				"it in that weapon slot, an empty slot wears it. Use before an attack " +
				"or when describing what the character now holds." + factsNote,
			Schema: objSchema(map[string]any{
				"actor": strProp("Character equipping the item."),
				"item":  strProp("Item ID (\"item-3\") or inventory name."),
				"slot":  strProp("Weapon slot to wield into (\"main\", \"off\", …); empty string wears the item instead."),
			}, "actor", "item"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor string `json:"actor"`
					Item  string `json:"item"`
					Slot  string `json:"slot"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				for _, f := range []struct{ name, value string }{
					{"actor", a.Actor}, {"item", a.Item},
				} {
					if err := requireField(f.name, f.value); err != nil {
						return Result{}, err
					}
				}
				// Slot is optional: an absent or empty slot wears the item
				// (resolve.Equip's own default), a named slot wields it.
				res, err := resolve.Apply(s, resolve.Equip{
					Actor: a.Actor, Item: a.Item, Slot: a.Slot,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "check_encumbrance",
			Description: "Report the advisory carried-item count for a character. " +
				"ADVISORY ONLY: Mythras Imperative has no encumbrance rules — nothing " +
				"is penalised; this is GM judgement input for narration." + factsNote,
			Schema: objSchema(map[string]any{
				"actor": strProp("Character ID whose load is checked."),
			}, "actor"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor string `json:"actor"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				carried, err := resolve.CheckCarrying(s, a.Actor)
				if err != nil {
					return Result{}, err
				}
				return Result{
					OK:      true,
					Summary: fmt.Sprintf("%s carries %d item(s) (advisory)", a.Actor, carried),
					Facts: []string{
						fmt.Sprintf("carried items: %d", carried),
						"advisory: Mythras Imperative defines no encumbrance rules (digest §8) — no mechanical effect",
					},
				}, nil
			},
		},
	}
}
