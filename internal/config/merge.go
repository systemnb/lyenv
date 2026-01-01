package config

import "strings"

type MergeStrategy string

const (
	MergeOverride MergeStrategy = "override" // overlay replaces base
	MergeAppend   MergeStrategy = "append"   // maps: deep-merge; arrays: concatenate; scalars: overlay replaces
	MergeKeep     MergeStrategy = "keep"     // keep base; overlay fills only missing keys
)

func ParseMergeStrategy(s string) MergeStrategy {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "append":
		return MergeAppend
	case "keep":
		return MergeKeep
	case "override", "":
		return MergeOverride
	default:
		return MergeOverride
	}
}

func MergeMapWithStrategy(base, overlay map[string]interface{}, strategy MergeStrategy) map[string]interface{} {
	// copy base to avoid mutating input
	if base == nil {
		base = make(map[string]interface{})
	}
	if overlay == nil {
		return base
	}

	for k, ov := range overlay {
		bv, exists := base[k]
		if !exists {
			// not exist -> always set
			base[k] = ov
			continue
		}

		bm, bIsMap := bv.(map[string]interface{})
		om, oIsMap := ov.(map[string]interface{})
		ba, bIsArr := ToIfaceSlice(bv)
		oa, oIsArr := ToIfaceSlice(ov)

		switch strategy {
		case MergeKeep:
			// keep base; only add missing keys (already handled above)
			continue

		case MergeAppend:
			if bIsMap && oIsMap {
				base[k] = MergeMapWithStrategy(bm, om, strategy)
			} else if bIsArr && oIsArr {
				base[k] = append(ba, oa...) // concatenate arrays
			} else {
				// scalar or mismatched types -> overlay replaces
				base[k] = ov
			}

		case MergeOverride:
			if bIsMap && oIsMap {
				// Deep override for maps
				base[k] = MergeMapWithStrategy(bm, om, strategy)
			} else {
				// replace
				base[k] = ov
			}
		}
	}
	return base
}

func ToIfaceSlice(v interface{}) ([]interface{}, bool) {
	switch a := v.(type) {
	case []interface{}:
		return a, true
	default:
		return nil, false
	}
}
