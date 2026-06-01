package model

type Project struct {
	ProjectName   string            `json:"project_name"`
	SchemaVersion string            `json:"schema_version"`
	EngineVersion string            `json:"engine_version"`
	EntrySystem   string            `json:"entry_system"`
	Graph         string            `json:"graph"`
	Environment   EnvironmentConfig `json:"environment"`
	DefaultInput  string            `json:"default_input"`
	DefaultOutput string            `json:"default_output"`
}

type EnvironmentConfig struct {
	Mode     string `json:"mode"`
	Python   string `json:"python"`
	Lockfile string `json:"lockfile"`
}

type Graph struct {
	SchemaVersion string       `json:"schema_version"`
	Systems       []System     `json:"systems"`
	Components    []Component  `json:"components"`
	Connections   []Connection `json:"connections"`
}

type System struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Components    []string        `json:"components"`
	Connections   []string        `json:"connections"`
	PublicInputs  []PublicNodeRef `json:"public_inputs"`
	PublicOutputs []PublicNodeRef `json:"public_outputs"`
}

type PublicNodeRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Component string `json:"component"`
	Node      string `json:"node"`
	Medium    string `json:"medium"`
	ValueType string `json:"value_type"`
	Unit      string `json:"unit"`
	Required  *bool  `json:"required"`
	Default   any    `json:"default"`
}

func (r PublicNodeRef) IsRequired() bool {
	return r.Required == nil || *r.Required
}

type Component struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	Class      string         `json:"class"`
	Nodes      NodeSet        `json:"nodes"`
	Parameters map[string]any `json:"parameters"`
}

type NodeSet struct {
	Inputs  []Node `json:"inputs"`
	Outputs []Node `json:"outputs"`
}

type Node struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Medium    string `json:"medium"`
	ValueType string `json:"value_type"`
	Unit      string `json:"unit"`
	Required  *bool  `json:"required"`
	Default   any    `json:"default"`
}

func (n Node) IsRequired() bool {
	return n.Required == nil || *n.Required
}

type Connection struct {
	ID   string   `json:"id"`
	From Endpoint `json:"from"`
	To   Endpoint `json:"to"`
}

type Endpoint struct {
	Component string `json:"component"`
	Node      string `json:"node"`
}
