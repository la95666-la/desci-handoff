package action

type LoginRequest struct {
	Content string `json:"content"`
}

type LoginData struct {
	DeviceID string `json:"deviceID"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type TaskRequest struct {
	ID    string `json:"taskID"`
	Name  string `json:"taskName"`
	Token string `json:"token"`
}

type Monitor struct {
	TaskID         string `json:"taskID"`
	GpuName        string `json:"gpuName"`
	GpuType        string `json:"gpuType"`
	GpuMemory      int64  `json:"gpuMemory"`
	GpuUsed        int64  `json:"gpuUsed"`
	GpuTemperature int64  `json:"gpuTemperature"`
}

type AskForModuleUpdateRequest struct {
	ID string `json:"taskID"`
}

type Update struct {
	TaskID      string `json:"taskID"`
	TaskName    string `json:"taskName"`
	RuntimeType string `json:"runtimeType"`
	Token       string `json:"token"`
	Version     int    `json:"version"`
	Hash        string `json:"hash"`
	Url         string `json:"url"`
	NeedStart   bool   `json:"needStart"`
}
