package domain

import (
	"encoding/json"
	"time"
)

type SystemConfig struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Description *string         `json:"description,omitempty"`
	UpdatedBy   *string         `json:"updatedBy,omitempty"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// BoolValue parses the config value as a boolean.
func (c *SystemConfig) BoolValue() bool {
	var v bool
	_ = json.Unmarshal(c.Value, &v)
	return v
}

// Int64Value parses the config value as an int64.
func (c *SystemConfig) Int64Value() int64 {
	var v int64
	_ = json.Unmarshal(c.Value, &v)
	return v
}

// StringValue parses the config value as a string.
func (c *SystemConfig) StringValue() string {
	var v string
	_ = json.Unmarshal(c.Value, &v)
	return v
}
