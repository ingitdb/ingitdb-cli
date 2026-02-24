package ingitdb

// TriggerEventType is the lifecycle event that fires a trigger.
type TriggerEventType string

const (
	TriggerEventCreated TriggerEventType = "created"
	TriggerEventUpdated TriggerEventType = "updated"
	TriggerEventDeleted TriggerEventType = "deleted"
)

// TriggerStepDef is a single shell step within a trigger job.
type TriggerStepDef struct {
	Run string `yaml:"run"`
}

// TriggerJobDef is one job inside a trigger workflow.
// RunsOn must be "." (run in the same environment as ingitdb itself).
type TriggerJobDef struct {
	RunsOn string           `yaml:"runs-on"`
	Steps  []TriggerStepDef `yaml:"steps"`
}

// TriggerDef is the schema for a trigger workflow file
// (.ingitdb-collection/trigger_<name>.yaml).
// Modelled after GitHub Actions workflow syntax.
type TriggerDef struct {
	On   []TriggerEventType        `yaml:"on"`
	Jobs map[string]*TriggerJobDef `yaml:"jobs"`
}
