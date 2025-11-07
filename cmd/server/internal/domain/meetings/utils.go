package meetings

import (
	orchestrator "github.com/houzhh15/AIDG/cmd/server/internal/orchestrator"
)

// BackfillSBDefaults backfills default values for SpeechBrain parameters
// Returns true if defaults were applied
func BackfillSBDefaults(cfg *orchestrator.Config) bool {
	// Heuristic: treat as uninitialized only if ALL tunables are zero/false.
	if cfg.SBOverclusterFactor == 0 && cfg.SBMergeThreshold == 0 && cfg.SBMinSegmentMerge == 0 && !cfg.SBReassignAfterMerge && !cfg.SBEnergyVAD && cfg.SBEnergyVADThr == 0 {
		def := orchestrator.DefaultConfig()
		cfg.SBOverclusterFactor = def.SBOverclusterFactor
		cfg.SBMergeThreshold = def.SBMergeThreshold
		cfg.SBMinSegmentMerge = def.SBMinSegmentMerge
		cfg.SBReassignAfterMerge = def.SBReassignAfterMerge
		cfg.SBEnergyVAD = def.SBEnergyVAD
		cfg.SBEnergyVADThr = def.SBEnergyVADThr
		return true
	}
	return false
}
