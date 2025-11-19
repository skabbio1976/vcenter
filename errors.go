package vcenter

import "fmt"

// Error types för bättre felhantering

// NotFoundError returneras när en resurs inte hittas
type NotFoundError struct {
	ResourceType string
	Name         string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.ResourceType, e.Name)
}

// ValidationError returneras när input-validering misslyckas
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

// OperationError returneras när en vCenter-operation misslyckas
type OperationError struct {
	Operation string
	Err       error
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("operation %s failed: %v", e.Operation, e.Err)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

// ConfigurationError returneras när konfiguration är felaktig
type ConfigurationError struct {
	Message string
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error: %s", e.Message)
}
