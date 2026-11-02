package npmprofile

import "fmt"

type publishObservationSummary struct {
	expectedSRI   string
	first         string
	latestChanged string
	final         string
	observations  int
	changes       int
}

func newPublishObservationSummary(expectedSRI string) *publishObservationSummary {
	return &publishObservationSummary{expectedSRI: expectedSRI}
}

func (summary *publishObservationSummary) record(observation registryObservation) {
	packageState := "unknown"
	versionState := "unknown"
	integrityState := "unknown"
	if observation.packumentErr == nil {
		packageState = "absent"
		if observation.packageExists {
			packageState = "present"
		}
		versionState = "absent"
		if observation.version != nil {
			versionState = "present"
			normalized, valid := normalizeSHA512SRI(observation.version.Integrity)
			switch {
			case !valid:
				integrityState = "malformed"
			case normalized == summary.expectedSRI:
				integrityState = "match"
			default:
				integrityState = "mismatch"
			}
		}
	}

	attestationState := "absent"
	if observation.attestationErr != nil {
		attestationState = "unknown"
	} else if observation.attestations.Found {
		attestationState = "present"
	}

	errorState := "none"
	switch {
	case observation.packumentErr != nil && observation.attestationErr != nil:
		errorState = "both"
	case observation.packumentErr != nil:
		errorState = "packument"
	case observation.attestationErr != nil:
		errorState = "attestation"
	}

	current := fmt.Sprintf(
		"package=%s version=%s integrity=%s attestations=%s errors=%s",
		packageState, versionState, integrityState, attestationState, errorState,
	)
	summary.observations++
	if summary.observations == 1 {
		summary.first = current
	} else if current != summary.first && current != summary.latestChanged {
		summary.changes++
		summary.latestChanged = current
	}
	summary.final = current
}

func (summary publishObservationSummary) format(afterMutation bool, decision PublishState) string {
	line := fmt.Sprintf(
		"readback after_mutation=%t observations=%d changes=%d first=%q ",
		afterMutation, summary.observations, summary.changes, summary.first,
	)
	if summary.latestChanged != "" {
		line += fmt.Sprintf("latest_changed=%q ", summary.latestChanged)
	}
	return line + fmt.Sprintf("final=%q decision=%s", summary.final, decision)
}
