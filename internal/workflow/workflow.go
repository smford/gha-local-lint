package workflow

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/asc/gha-local-lint/internal/matrix"
	"gopkg.in/yaml.v3"
)

type Position struct {
	Line   int
	Column int
}

type Workflow struct {
	Path        string
	Name        string
	RawNode     *yaml.Node
	On          *TriggerConfig
	Permissions *PermissionsConfig
	Env         map[string]string
	Jobs        map[string]*Job
	JobOrder    []string // preserves order in file
	Pos         Position
}

type TriggerConfig struct {
	Events map[string]*EventFilter // "push", "pull_request", "workflow_call", etc.
	Inputs map[string]*WorkflowInput
	Pos    Position
}

type WorkflowInput struct {
	Required    bool
	Default     any
	Type        string
	Description string
	Pos         Position
}

type EventFilter struct {
	Branches       []string
	BranchesIgnore []string
	Paths          []string
	PathsIgnore    []string
	Tags           []string
	Types          []string
	Pos            Position
}

type PermissionsConfig struct {
	IsRawString bool              // e.g. "read-all" or "write-all"
	RawValue    string            // "write-all", "read-all"
	Scopes      map[string]string // e.g. "contents": "read", "issues": "write"
	Pos         Position
}

type Job struct {
	ID             string
	Name           string
	RunsOn         any
	Needs          []string
	NeedsPos       map[string]Position
	If             string
	IfPos          Position
	Permissions    *PermissionsConfig
	TimeoutMinutes *int
	TimeoutPos     Position
	Strategy       *matrix.Strategy
	Outputs        map[string]JobOutput
	Env            map[string]string
	Uses           string
	UsesPos        Position
	With           map[string]any
	Secrets        map[string]any
	Steps          []*Step
	Pos            Position
}

type JobOutput struct {
	Value string
	Pos   Position
}

type Step struct {
	ID      string
	Name    string
	Uses    string
	UsesPos Position
	Run     string
	RunPos  Position
	If      string
	IfPos   Position
	Env     map[string]string
	With    map[string]any
	Pos     Position
}

// ParseWorkflow parses a workflow file into a Workflow struct with line/column metadata.
func ParseWorkflow(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file: %w", err)
	}
	return ParseWorkflowBytes(path, data)
}

func ParseWorkflowBytes(path string, data []byte) (*Workflow, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if len(root.Content) == 0 {
		return nil, fmt.Errorf("empty YAML document")
	}

	docNode := root.Content[0]
	if docNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping at root of workflow, got kind %d", docNode.Kind)
	}

	wf := &Workflow{
		Path:    path,
		RawNode: &root,
		Jobs:    make(map[string]*Job),
		Env:     make(map[string]string),
		Pos:     Position{Line: docNode.Line, Column: docNode.Column},
	}

	for i := 0; i < len(docNode.Content); i += 2 {
		keyNode := docNode.Content[i]
		valNode := docNode.Content[i+1]

		switch keyNode.Value {
		case "name":
			wf.Name = valNode.Value
		case "on":
			wf.On = parseTriggerConfig(valNode)
		case "permissions":
			wf.Permissions = parsePermissions(valNode)
		case "env":
			wf.Env = parseEnv(valNode)
		case "jobs":
			if valNode.Kind == yaml.MappingNode {
				for j := 0; j < len(valNode.Content); j += 2 {
					jobKeyNode := valNode.Content[j]
					jobValNode := valNode.Content[j+1]
					jobID := jobKeyNode.Value
					job := parseJob(jobID, jobValNode)
					wf.Jobs[jobID] = job
					wf.JobOrder = append(wf.JobOrder, jobID)
				}
			}
		}
	}

	return wf, nil
}

func parseTriggerConfig(node *yaml.Node) *TriggerConfig {
	tc := &TriggerConfig{
		Events: make(map[string]*EventFilter),
		Inputs: make(map[string]*WorkflowInput),
		Pos:    Position{Line: node.Line, Column: node.Column},
	}

	switch node.Kind {
	case yaml.ScalarNode:
		// on: push
		tc.Events[node.Value] = &EventFilter{Pos: Position{Line: node.Line, Column: node.Column}}
	case yaml.SequenceNode:
		// on: [push, pull_request]
		for _, item := range node.Content {
			tc.Events[item.Value] = &EventFilter{Pos: Position{Line: item.Line, Column: item.Column}}
		}
	case yaml.MappingNode:
		// on:
		//   push:
		//     branches: [main]
		for i := 0; i < len(node.Content); i += 2 {
			evtKey := node.Content[i].Value
			evtVal := node.Content[i+1]
			filter := &EventFilter{Pos: Position{Line: node.Content[i].Line, Column: node.Content[i].Column}}
			if evtVal.Kind == yaml.MappingNode {
				for j := 0; j < len(evtVal.Content); j += 2 {
					k := evtVal.Content[j].Value
					v := evtVal.Content[j+1]
					switch k {
					case "branches":
						filter.Branches = parseStringList(v)
					case "branches-ignore":
						filter.BranchesIgnore = parseStringList(v)
					case "paths":
						filter.Paths = parseStringList(v)
					case "paths-ignore":
						filter.PathsIgnore = parseStringList(v)
					case "tags":
						filter.Tags = parseStringList(v)
					case "types":
						filter.Types = parseStringList(v)
					case "inputs":
						if v.Kind == yaml.MappingNode {
							for m := 0; m < len(v.Content); m += 2 {
								inName := v.Content[m].Value
								inVal := v.Content[m+1]
								wfInput := &WorkflowInput{Pos: Position{Line: v.Content[m].Line, Column: v.Content[m].Column}}
								if inVal.Kind == yaml.MappingNode {
									for n := 0; n < len(inVal.Content); n += 2 {
										prop := inVal.Content[n].Value
										propVal := inVal.Content[n+1]
										switch prop {
										case "required":
											wfInput.Required = strings.ToLower(propVal.Value) == "true"
										case "type":
											wfInput.Type = propVal.Value
										case "description":
											wfInput.Description = propVal.Value
										}
									}
								}
								tc.Inputs[inName] = wfInput
							}
						}
					}
				}
			}
			tc.Events[evtKey] = filter
		}
	}
	return tc
}

func parsePermissions(node *yaml.Node) *PermissionsConfig {
	pc := &PermissionsConfig{
		Scopes: make(map[string]string),
		Pos:    Position{Line: node.Line, Column: node.Column},
	}
	if node.Kind == yaml.ScalarNode {
		pc.IsRawString = true
		pc.RawValue = node.Value
		return pc
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			k := node.Content[i].Value
			v := node.Content[i+1].Value
			pc.Scopes[k] = v
		}
	}
	return pc
}

func parseEnv(node *yaml.Node) map[string]string {
	res := make(map[string]string)
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			res[node.Content[i].Value] = node.Content[i+1].Value
		}
	}
	return res
}

func parseJob(id string, node *yaml.Node) *Job {
	job := &Job{
		ID:       id,
		NeedsPos: make(map[string]Position),
		Outputs:  make(map[string]JobOutput),
		Env:      make(map[string]string),
		Pos:      Position{Line: node.Line, Column: node.Column},
	}

	if node.Kind != yaml.MappingNode {
		return job
	}

	for i := 0; i < len(node.Content); i += 2 {
		kNode := node.Content[i]
		vNode := node.Content[i+1]

		switch kNode.Value {
		case "name":
			job.Name = vNode.Value
		case "runs-on":
			if vNode.Kind == yaml.ScalarNode {
				job.RunsOn = vNode.Value
			} else if vNode.Kind == yaml.SequenceNode {
				var list []string
				for _, item := range vNode.Content {
					list = append(list, item.Value)
				}
				job.RunsOn = list
			}
		case "needs":
			if vNode.Kind == yaml.ScalarNode {
				job.Needs = append(job.Needs, vNode.Value)
				job.NeedsPos[vNode.Value] = Position{Line: vNode.Line, Column: vNode.Column}
			} else if vNode.Kind == yaml.SequenceNode {
				for _, item := range vNode.Content {
					job.Needs = append(job.Needs, item.Value)
					job.NeedsPos[item.Value] = Position{Line: item.Line, Column: item.Column}
				}
			}
		case "if":
			job.If = vNode.Value
			job.IfPos = Position{Line: vNode.Line, Column: vNode.Column}
		case "permissions":
			job.Permissions = parsePermissions(vNode)
		case "timeout-minutes":
			if val, err := strconv.Atoi(vNode.Value); err == nil {
				job.TimeoutMinutes = &val
			}
			job.TimeoutPos = Position{Line: kNode.Line, Column: kNode.Column}
		case "strategy":
			job.Strategy = parseStrategy(vNode)
		case "outputs":
			if vNode.Kind == yaml.MappingNode {
				for j := 0; j < len(vNode.Content); j += 2 {
					outKey := vNode.Content[j].Value
					outVal := vNode.Content[j+1].Value
					job.Outputs[outKey] = JobOutput{
						Value: outVal,
						Pos:   Position{Line: vNode.Content[j].Line, Column: vNode.Content[j].Column},
					}
				}
			}
		case "uses":
			job.Uses = vNode.Value
			job.UsesPos = Position{Line: vNode.Line, Column: vNode.Column}
		case "with":
			var withMap map[string]any
			if err := vNode.Decode(&withMap); err == nil {
				job.With = withMap
			}
		case "secrets":
			var secMap map[string]any
			if err := vNode.Decode(&secMap); err == nil {
				job.Secrets = secMap
			}
		case "env":
			job.Env = parseEnv(vNode)
		case "steps":
			if vNode.Kind == yaml.SequenceNode {
				for _, stepNode := range vNode.Content {
					job.Steps = append(job.Steps, parseStep(stepNode))
				}
			}
		}
	}

	return job
}

func parseStep(node *yaml.Node) *Step {
	step := &Step{
		Env:  make(map[string]string),
		With: make(map[string]any),
		Pos:  Position{Line: node.Line, Column: node.Column},
	}

	if node.Kind != yaml.MappingNode {
		return step
	}

	for i := 0; i < len(node.Content); i += 2 {
		kNode := node.Content[i]
		vNode := node.Content[i+1]

		switch kNode.Value {
		case "id":
			step.ID = vNode.Value
		case "name":
			step.Name = vNode.Value
		case "uses":
			step.Uses = vNode.Value
			step.UsesPos = Position{Line: vNode.Line, Column: vNode.Column}
		case "run":
			step.Run = vNode.Value
			step.RunPos = Position{Line: vNode.Line, Column: vNode.Column}
		case "if":
			step.If = vNode.Value
			step.IfPos = Position{Line: vNode.Line, Column: vNode.Column}
		case "env":
			step.Env = parseEnv(vNode)
		case "with":
			// Unmarshal generic map
			var withMap map[string]any
			if err := vNode.Decode(&withMap); err == nil {
				step.With = withMap
			}
		}
	}

	return step
}

func parseStrategy(node *yaml.Node) *matrix.Strategy {
	strat := &matrix.Strategy{
		Dimensions: make(map[string][]any),
	}

	if node.Kind == yaml.ScalarNode {
		strat.Expression = node.Value
		return strat
	}

	if node.Kind != yaml.MappingNode {
		return strat
	}

	for i := 0; i < len(node.Content); i += 2 {
		k := node.Content[i].Value
		vNode := node.Content[i+1]

		switch k {
		case "fail-fast":
			val := strings.ToLower(vNode.Value) == "true"
			strat.FailFast = &val
		case "max-parallel":
			if val, err := strconv.Atoi(vNode.Value); err == nil {
				strat.MaxParallel = val
			}
		case "matrix":
			if vNode.Kind == yaml.ScalarNode {
				strat.Expression = vNode.Value
			} else if vNode.Kind == yaml.MappingNode {
				for j := 0; j < len(vNode.Content); j += 2 {
					mKey := vNode.Content[j].Value
					mVal := vNode.Content[j+1]

					if mKey == "include" {
						var incList []map[string]any
						_ = mVal.Decode(&incList)
						strat.Include = incList
					} else if mKey == "exclude" {
						var excList []map[string]any
						_ = mVal.Decode(&excList)
						strat.Exclude = excList
					} else {
						// Dimension array
						var items []any
						if mVal.Kind == yaml.SequenceNode {
							for _, it := range mVal.Content {
								var parsed any
								_ = it.Decode(&parsed)
								items = append(items, parsed)
							}
						}
						strat.Dimensions[mKey] = items
					}
				}
			}
		}
	}

	return strat
}

func parseStringList(node *yaml.Node) []string {
	var list []string
	if node.Kind == yaml.ScalarNode {
		list = append(list, node.Value)
	} else if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			list = append(list, item.Value)
		}
	}
	return list
}
