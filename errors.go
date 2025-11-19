package vcenter

import "fmt"

// Error types for better error handling

// NotFoundError is returned when a resource is not found
type NotFoundError struct {
	ResourceType string
	Name         string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.ResourceType, e.Name)
}

// ValidationError is returned when input validation fails
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

// OperationError is returned when a vCenter operation fails
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

// ConfigurationError is returned when configuration is invalid
type ConfigurationError struct {
	Message string
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error: %s", e.Message)
}
