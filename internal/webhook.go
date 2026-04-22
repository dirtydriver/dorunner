package internal

type WebhookPayload struct {
	Action      string      `json:"action"`
	Repository  Repository  `json:"repository"`
	WorkflowJob WorkflowJob `json:"workflow_job"`
}

type WorkflowJob struct {
	Labels     []string `json:"labels"`
	ID         int64    `json:"id"`
	RunnerName string   `json:"runner_name"`
}

type Repository struct {
	FullName string `json:"full_name"`
}
