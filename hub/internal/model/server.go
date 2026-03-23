package model

import "time"

type Server struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Hostname          *string   `json:"hostname"`
	IPAddress         *string   `json:"ip_address"`
	OS                *string   `json:"os"`
	Status            string    `json:"status"`
	RegistrationToken *string   `json:"-"`
	TokenExpiresAt    *string   `json:"-"`
	AgentVersion      *string   `json:"agent_version"`
	Labels            string    `json:"labels"`
	LastSeenAt        *string   `json:"last_seen_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ServerFilter struct {
	Status *string
	Search *string
	Limit  int
	Offset int
}

type CreateServerRequest struct {
	Name   string `json:"name"`
	Labels string `json:"labels,omitempty"`
}

type CreateServerResponse struct {
	Server            Server `json:"server"`
	RegistrationToken string `json:"registration_token"`
	InstallCommand    string `json:"install_command"`
}

type RegisterAgentRequest struct {
	Token        string `json:"token"`
	Hostname     string `json:"hostname"`
	IPAddress    string `json:"ip_address"`
	OS           string `json:"os"`
	AgentVersion string `json:"agent_version"`
}

type BatchDeleteRequest struct {
	IDs []string `json:"ids"`
}
