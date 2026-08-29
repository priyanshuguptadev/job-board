package domain

import (
	"context"
	"time"
)

// ApplicationStage represents the stage of an application in the hiring pipeline.
type ApplicationStage string

const (
	ApplicationStageApplied      ApplicationStage = "applied"
	ApplicationStageScreening    ApplicationStage = "screening"
	ApplicationStageInterviewing ApplicationStage = "interviewing"
	ApplicationStageOffer        ApplicationStage = "offer"
	ApplicationStageHired        ApplicationStage = "hired"
	ApplicationStageRejected     ApplicationStage = "rejected"
)

// Application represents a candidate's submission for a job posting.
type Application struct {
	ID             string                 `json:"id"`
	JobID          string                 `json:"job_id"`
	CandidateName  string                 `json:"candidate_name"`
	CandidateEmail string                 `json:"candidate_email"`
	CandidatePhone *string                `json:"candidate_phone,omitempty"`
	LinkedInURL    *string                `json:"linkedin_url,omitempty"`
	ResumeS3Key    string                 `json:"resume_s3_key"`
	ResumeFilename string                 `json:"resume_filename"`
	CustomAnswers  map[string]interface{} `json:"custom_answers"`
	Stage          ApplicationStage       `json:"stage"`
	RejectedReason *string                `json:"rejected_reason,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// ApplicationListFilter defines filtering and pagination parameters for querying applications.
type ApplicationListFilter struct {
	JobID          string
	Stage          ApplicationStage
	CandidateEmail string
	Page           int
	Limit          int
}

// ApplicationRepository defines persistence operations for candidate applications.
type ApplicationRepository interface {
	Create(ctx context.Context, app *Application) error
	GetByID(ctx context.Context, id string) (*Application, error)
	List(ctx context.Context, filter ApplicationListFilter) ([]*Application, int, error)
	Update(ctx context.Context, app *Application) error
	UpdateStage(ctx context.Context, id string, stage ApplicationStage, rejectedReason *string) error
	Delete(ctx context.Context, id string) error
}
