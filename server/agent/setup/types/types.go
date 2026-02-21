package types

// SetupRequest is sent to the server to register a new machine
type SetupRequest struct {
	Name      string            `json:"name"`
	Hostname  string            `json:"hostname"`
	OS        string            `json:"os"`
	Arch      string            `json:"arch"`
	CPUs      int               `json:"cpus"`
	IP        string            `json:"ip"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	ClaimCode string            `json:"claim_code,omitempty"` // optional; links machine to user
}

// SetupResponse is returned by the server after registration
type SetupResponse struct {
	MachineID   string `json:"machine_id"`
	GRPCAddress string `json:"grpc_address"`
	Message     string `json:"message"`
}

// PendingCmd represents a command received from the server config
type PendingCmd struct {
	ID      string   `json:"id"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}
