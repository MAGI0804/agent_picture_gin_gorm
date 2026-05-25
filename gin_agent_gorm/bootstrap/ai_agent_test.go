package bootstrap

import (
	"reflect"
	"testing"

	"gin-biz-web-api/model"
)

func TestAIAgentAutoMigrateModelsIncludesV2FirstRoundTables(t *testing.T) {
	models := aiAgentAutoMigrateModels()

	required := []reflect.Type{
		reflect.TypeOf(&model.AgentRun{}),
		reflect.TypeOf(&model.AgentStep{}),
		reflect.TypeOf(&model.ContextMemory{}),
		reflect.TypeOf(&model.Artifact{}),
		reflect.TypeOf(&model.ArtifactVersion{}),
		reflect.TypeOf(&model.ArtifactFeedback{}),
		reflect.TypeOf(&model.TaskLedgerItem{}),
		reflect.TypeOf(&model.ToolInvocation{}),
		reflect.TypeOf(&model.MemoryEvent{}),
	}

	for _, requiredType := range required {
		t.Run(requiredType.Elem().Name(), func(t *testing.T) {
			if !hasMigrateModel(models, requiredType) {
				t.Fatalf("aiAgentAutoMigrateModels() does not include %s", requiredType)
			}
		})
	}
}

func hasMigrateModel(models []interface{}, requiredType reflect.Type) bool {
	for _, candidate := range models {
		if reflect.TypeOf(candidate) == requiredType {
			return true
		}
	}
	return false
}
