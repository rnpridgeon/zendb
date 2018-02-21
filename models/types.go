package models

import (
	"encoding/json"
	"time"
)

type (
	Utime              int64 // Rather than dealing with timezones, formatting and such all times are represented as unix epoch
	calendarTime       int64 // Calendar time is the only time that matters
	TicketData         CustomFieldValue
	OrganizationData   CustomFieldValue
	UserData           CustomFieldValue
	TicketFields       CustomField
	OrganizationFields CustomField
	UserFields         CustomField
)

func (u *Utime) UnmarshalJSON(b []byte) error {
	var tmp time.Time
	*u = 0

	err := json.Unmarshal(b, &tmp)

	if (Utime(tmp.Unix())) > 0 {
		*u = Utime(tmp.Unix())
		return err
	}

	return err
}

func (c *calendarTime) UnmarshalJSON(b []byte) error {
	var tmp businessCalendar
	err := json.Unmarshal(b, &tmp)
	*c = calendarTime(tmp.Calendar)
	return err
}

type CustomField struct {
	Id            int64                  `json:"id,omitempty" structs:",isKey"`
	Type          string                 `json:"type,omitempty"`
	SKey          string                 `json:"key,omitempty"`
	Title         string                 `json:"title,omitempty"`
	CreatedAt     Utime                  `json:"created_at,omitempty"`
	UpdatedAt     Utime                  `json:"updated_at,omitempty"`
	SystemOptions map[string]interface{} `json:"-" structs:"-"`
	CustomOptions map[string]string      `json:"-" structs:"-"`
}

// Notes: resource type: Embedded, Foreign key/value to [resource]_fields
// custom_fields - key/value pair for custom fields
type CustomFieldValue struct {
	ObjectID    int64       `json:",omitempty" structs:",isKey"`
	Id          int64       `json:"id,omitempty" structs:",isKey"`
	Title       string      `json:",omitempty"`
	Value       interface{} `json:"value,omitempty"`
	Transformed string      `json:",omitempty"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/tickets#content
// Parent: root
// Notes: resource type: Data
// tickets - basic ticket object
type Ticket struct {
	Id                 int64              `json:"id,omitempty" structs:",isKey"`
	ExternalId         int64              `json:"external_id,omitempty"`
	Subject            string             `json:"subject,omitempty"`
	Status             string             `json:"status,omitempty"`
	Recipient          string             `json:"recipient,omitempty"`
	RequesterId        int64              `json:"requester_id,omitempty"`
	SubmitterId        int64              `json:"submitter_id,omitempty"`
	AssigneeId         int64              `json:"assignee_id,omitempty"`
	OrganizationId     int64              `json:"organization_id,omitempty"`
	GroupId            int64              `json:"group_id,omitempty"`
	CustomFields       []TicketData       `json:"custom_fields,omitempty" structs:"-"`
	SatisfactionRating SatisfactionRating `json:"satisfaction_rating,omitempty"  structs:"-"`
	CreatedAt          Utime              `json:"created_at,omitempty"`
	UpdatedAt          Utime              `json:"updated_at,omitempty"`
}

type TicketEnhanced struct {
	Ticket
	Version   string `json:"version,omitempty"`
	Component string `json:"component,omitempty"`
	Priority  string `json:"priority,omitempty"`
	TTFR      int64  `json:"ttfr,omitempty"`
	Solved_at Utime  `json:"solved_at,omitempty"`
}

// Doc:https://developer.zendesk.com/rest_api/docs/core/ticket_metrics
// Parent: tickets
// Notes: resource type: Data
// ticket_metrics - ticket life-cycle metrics
type TicketMetric struct {
	Id                   int64        `json:"id,omitempty" structs:",isKey"`
	TicketId             int64        `json:"ticket_id,omitempty"`
	Reopens              int64        `json:"reopens,omitempty"`
	Replies              int64        `json:"replies,omitempty"`
	AssigneeUpdatedAt    Utime        `json:"assignee_updated_at,omitempty"`
	RequesterUpdatedAt   Utime        `json:"requester_updated_at,omitempty"`
	StatusUpdatedAt      Utime        `json:"status_updated_at,omitempty"`
	InitiallyAssignedAt  Utime        `json:"initially_assigned_at,omitempty"`
	AssignedAt           Utime        `json:"assigned_at,omitempty"`
	SolvedAt             Utime        `json:"solved_at,omitempty"`
	LatestCommentAddedAt Utime        `json:"latest_comment_added_at,omitempty"`
	TTFR                 calendarTime `json:"reply_time_in_minutes,omitempty"`
	TTR                  calendarTime `json:"full_resolution_time_in_minutes,omitempty"`
	AgentWaitTime        calendarTime `json:"agent_wait_time_in_minutes,omitempty"`
	RequesterWaitTime    calendarTime `json:"requester_wait_time_in_minutes,omitempty"`
	CreatedAt            Utime        `json:"created_at,omitempty"`
	UpdatedAt            Utime        `json:"updated_at,omitempty"`
}

// Doc: derived from example in ticket_metrics, no direct documentation found
// Parent: ticket_metrics
// Notes: resource type: Embedded
// businessCalendar - helper structure showing time in minutes for both calendar and business days
type businessCalendar struct {
	Calendar int64 `json:"calendar,omitempty"`
	Business int64 `json:"business,omitempty"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/ticket_audits#content
// Parent: tickets
// Notes: resource type: Data
// audit - ticket changelog
type Audit struct {
	Id        int64         `json:"id,omitempty" structs:",isKey"`
	TicketId  int64         `json:"ticket_id,omitempty" structs:",isKey"`
	CreatedAt Utime         `json:"created_at,omitempty"`
	AuthorId  int64         `json:"author_id,omitempty"`
	Events    []ChangeEvent `json:"events,omitempty" structs:"value"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/ticket_audits#audit-events
// Parent: audit
// Notes: resource : Data; Currently only `Create` and `Change` events are supported, this Union represents both
// event - change metadata
type Event struct {
	Id        int64       `json:"id,omitempty"`
	Type      string      `json:"type,omitempty"`
	FieldName string      `json:"field_name,omitempty"`
	Value     interface{} `json:"value,omitempty"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/ticket_audits#audit-events
// Parent: audit
// Notes: resource type: Data; Currently only `Create` and `Change` events are supported, this Union represents both
// event - change metadata
type ChangeEvent struct {
	Event
	PValue interface{} `json:"previous_value,omitempty"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/satisfaction_ratings
// Parent: ticket
// Notes: resource type: Embedded
// satisfaction_rating - survey response data
type SatisfactionRating struct {
	Id          int64  `json:"id,omitempty"`
	URL         string `json:"url,omitempty"`
	AssigneeId  int64  `json:"assignee_id,omitempty"`
	GroupId     int64  `json:"group_id,omitempty"`
	RequesterId int64  `json:"requester_id,omitempty"`
	TicketId    int64  `json:"ticket_id,omitempty"`
	Score       string `json:"score,omitempty"`
	CreatedAt   Utime  `json:"created_at,omitempty"`
	UpdatedAt   Utime  `json:"updated_at,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/users
// Parent: root
// Notes: resource type: Data
// user - basic user object
type User struct {
	Id             int64                  `json:"id,omitempty" structs:",isKey"`
	Email          string                 `json:"email,omitempty"`
	Name           string                 `json:"name,omitempty"`
	CreatedAt      Utime                  `json:"created_at,omitempty"`
	ExternalId     string                 `json:"exteranl_id,omitempty"`
	LastLoginAt    Utime                  `json:"last_login_at,omitempty"`
	OrganizationId int64                  `json:"organization_id,omitempty"`
	GroupId        int64                  `json:"default_group_id,omitempty"`
	Role           string                 `json:"role,omitempty"`
	Suspended      bool                   `json:"suspended,omitempty"`
	TimeZone       string                 `json:"time_zone,omitempty"`
	UpdatedAt      Utime                  `json:"updated_at,omitempty"`
	CustomFields   map[string]interface{} `json:"user_fields,omitempty" structs:"-"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/organizations
// Parent: root
// Notes: resource type: Data
// organization - basic organization object
type Organization struct {
	Id           int64                  `json:"id,omitempty" structs:",isKey"`
	Name         string                 `json:"name,omitempty"`
	CreatedAt    Utime                  `json:"created_at,omitempty"`
	UpdatedAt    Utime                  `json:"updated_at,omitempty"`
	GroupId      int64                  `json:"group_id,omitempty"`
	CustomFields map[string]interface{} `json:"organization_fields,omitempty" structs:"-"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/groups
// Parent: root
// Notes: resource type: Data
// group - ticket working group
type Groups struct {
	Id        int64  `json:"id,omitempty" structs:",isKey"`
	Name      string `json:"name,omitempty"`
	CreatedAt Utime  `json:"created_at,omitempty"`
	UpdatedAt Utime  `json:"updated_at,omitempty"`
}
