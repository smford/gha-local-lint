package rules

import (
	"encoding/json"
	"fmt"
)

type SarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []SarifRun `json:"runs"`
}

type SarifRun struct {
	Tool    SarifTool     `json:"tool"`
	Results []SarifResult `json:"results"`
}

type SarifTool struct {
	Driver SarifDriver `json:"driver"`
}

type SarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []SarifRule `json:"rules"`
}

type SarifRule struct {
	ID               string               `json:"id"`
	ShortDescription SarifMultiformatText `json:"shortDescription"`
	FullDescription  SarifMultiformatText `json:"fullDescription,omitempty"`
	DefaultConfig    SarifDefaultConfig   `json:"defaultConfiguration"`
}

type SarifDefaultConfig struct {
	Level string `json:"level"`
}

type SarifResult struct {
	RuleID    string               `json:"ruleId"`
	Level     string               `json:"level"`
	Message   SarifMultiformatText `json:"message"`
	Locations []SarifLocation      `json:"locations"`
}

type SarifLocation struct {
	PhysicalLocation SarifPhysicalLocation `json:"physicalLocation"`
}

type SarifPhysicalLocation struct {
	ArtifactLocation SarifArtifactLocation `json:"artifactLocation"`
	Region           SarifRegion           `json:"region"`
}

type SarifArtifactLocation struct {
	URI string `json:"uri"`
}

type SarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
}

type SarifMultiformatText struct {
	Text string `json:"text"`
}

func FormatSARIF(findings []Finding, versionStr string) ([]byte, error) {
	ruleMap := make(map[string]SarifRule)

	var results []SarifResult
	for _, f := range findings {
		level := "warning"
		if f.Severity == SeverityError {
			level = "error"
		} else if f.Severity == SeverityInfo {
			level = "note"
		}

		if _, exists := ruleMap[f.Rule]; !exists {
			ruleMap[f.Rule] = SarifRule{
				ID: f.Rule,
				ShortDescription: SarifMultiformatText{
					Text: fmt.Sprintf("Rule %s", f.Rule),
				},
				DefaultConfig: SarifDefaultConfig{
					Level: level,
				},
			}
		}

		res := SarifResult{
			RuleID: f.Rule,
			Level:  level,
			Message: SarifMultiformatText{
				Text: f.Message,
			},
			Locations: []SarifLocation{
				{
					PhysicalLocation: SarifPhysicalLocation{
						ArtifactLocation: SarifArtifactLocation{
							URI: f.Path,
						},
						Region: SarifRegion{
							StartLine:   f.Line,
							StartColumn: f.Column,
						},
					},
				},
			},
		}
		results = append(results, res)
	}

	var rulesList []SarifRule
	for _, r := range ruleMap {
		rulesList = append(rulesList, r)
	}

	log := SarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []SarifRun{
			{
				Tool: SarifTool{
					Driver: SarifDriver{
						Name:           "gha-local-lint",
						Version:        versionStr,
						InformationURI: "https://github.com/asc/gha-local-lint",
						Rules:          rulesList,
					},
				},
				Results: results,
			},
		},
	}

	return json.MarshalIndent(log, "", "  ")
}
