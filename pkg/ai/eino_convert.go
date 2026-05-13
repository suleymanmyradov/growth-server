package ai

import (
	"github.com/cloudwego/eino/schema"
)

// Eino type aliases used internally for conversion.
type (
	einoMessage     = schema.Message
	einoToolCall    = schema.ToolCall
	einoFunctionCall = schema.FunctionCall
	einoRoleType    = schema.RoleType
)

const (
	einoSystem    = schema.System
	einoUser      = schema.User
	einoAssistant = schema.Assistant
	einoTool      = schema.Tool
)

// toEinoRole converts our Role to Eino's RoleType.
func toEinoRole(r Role) schema.RoleType {
	switch r {
	case RoleSystem:
		return schema.System
	case RoleUser:
		return schema.User
	case RoleAssistant:
		return schema.Assistant
	case RoleTool:
		return schema.Tool
	default:
		return schema.User
	}
}

// fromEinoRole converts Eino's RoleType to our Role.
func fromEinoRole(r schema.RoleType) Role {
	switch r {
	case schema.System:
		return RoleSystem
	case schema.User:
		return RoleUser
	case schema.Assistant:
		return RoleAssistant
	case schema.Tool:
		return RoleTool
	default:
		return RoleUser
	}
}
