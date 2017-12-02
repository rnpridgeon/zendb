package models

import (
	"time"
	"encoding/json"
)

// Rather than dealing with timezones, formatting and such all times are represented as unix epoch 
type utime int64

func (u *utime) UnmarshalJSON(b []byte) error {
	var tmp time.Time
	*u = 0

	err := json.Unmarshal(b, &tmp)

	if (utime(tmp.Unix())) > 0 {
		*u = utime(tmp.Unix())
		return err
	}

	return err
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/tickets#content
// Parent: root
// Notes: resource type: Data
// tickets - basic ticket object
type Ticket struct {
	Id                  int64                `json:"id"`
	Subject             string               `json:"subject"`
	Priority            string               `json:"priority"`
	Status              string               `json:"status"`
	Recipient           string               `json:"recipient"`
	Requester_id        int64                `json:"requester_id"`
	Submitter_id        int64                `json:"submitter_id"`
	Assignee_id         int64                `json:"assignee_id"`
	Organization_id     int64                `json:"organization_id"`
	Group_id            int64                `json:"group_id"`
	Custom_fields       []Custom_fields      `json:"custom_fields"`
	Satisfaction_rating *Satisfaction_rating `json:"satisfaction_rating"`
	Created_at          utime           `json:"created_at"`
	Updated_at          utime           `json:"updated_at"`
}

type Ticket_Enhanced struct {
	Ticket
	Version	string 				`json:"version"`
	Component string			`json:"component"`
	Priority string				`json:"priority"`
	TTFR	int64				`json:"ttfr"`
	Solved_at		utime		`json:"solved_at"`
}

// Doc: derived from example in ticket, no direct documentation found
// Parent: ticket
// Notes: resource type: Embedded, Foreign key/value to ticket_fields
// custom_fields - key/value pair for custom ticket fields
type Custom_fields struct {
	Id    int64       `json:"id"`
	Value interface{} `json:"value"`
	Transformed	string	`json:"-"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/ticket_fields
// Parent: root
// Notes: resource type: Metadata
// ticket_fields - customized ticket fields
type Ticket_field struct {
	Id                    int64                  `json:"id"`
	URL                   string                 `json:"url"`
	Type                  string                 `json:"type"`
	Title                 string                 `json:"title"`
	Raw_title             string                 `json:"raw_title"`
	Description           string                 `json:"description"`
	Position              int64                  `json:"position"`
	Active                bool                   `json:"active"`
	Required              bool                   `json:"required"`
	Collapsed_for_agents  bool                   `json:"collapsed_for_Agents"`
	Regexp_for_validation string                 `json:"regexp_for_validation"`
	Title_in_portal       string                 `json:"title_in_portal"`
	Raw_title_in_portal   string                 `json:"raw_title_in_portal"`
	Visible_in_portal     bool                   `json:"visible_in_portal"`
	Editable_in_portal    bool                   `json:"editable_in_portal"`
	Required_in_portal    bool                   `json:"required_in_portal"`
	Tag                   string                 `json:"tag"`
	Created_at            utime         	    `json:"created_at"`
	Updated_at            utime            		 `json:"updated_at"`
	System_field_options  map[string]interface{} `json:"-"`
	Custom_field_options  map[string]string      `json:"-"`
	Removable             bool                   `json:"removable"`
}

// Doc:https://developer.zendesk.com/rest_api/docs/core/ticket_metrics
// Parent: tickets
// Notes: resource type: Data
// ticket_metrics - ticket life-cycle metrics
type Ticket_metrics struct {
	Id                               int64              `json:"id"`
	Ticket_id                        int64              `json:"ticket_id"`
	URL                              string             `json:"url"`
	Group_statisons                  int64              `json:"group_stations"`
	Assignee_stations                int64              `json:"assignee_stations"`
	Reopens                          int64              `json:"reopens"`
	Replies                          int64              `json:"replies"`
	Assignee_updated_at              utime         		`json:"assignee_updated_at"`
	Requester_updated_at             utime        		`json:"requester_updated_at"`
	Status_updated_at                utime         		`json:"status_updated_at"`
	Initially_assigned_at            utime         		`json:"initially_assigned_at"`
	Assigned_at                      utime         		`json:"assigned_at"`
	Solved_at                        utime         		`json:"solved_at"`
	Latest_comment_added_at          utime         		`json:"latest_comment_added_at"`
	First_resolution_time_in_minutes *business_calendar `json:"first_resolution_time_in_minutes"`
	Reply_time_in_minutes            *business_calendar `json:"reply_time_in_minutes"`
	Full_resolution_time_in_minutes  *business_calendar `json:"full_resolution_time_in_minutes"`
	Agent_wait_time_in_minutes       *business_calendar `json:"agent_wait_time_in_minutes"`
	Requester_wait_time_in_minutes   *business_calendar `json:"requester_wait_time_in_minutes"`
	Created_at                       utime         		`json:"created_at"`
	Updated_at                       utime         		`json:"updated_at"`
}

// Doc: derived from example in ticket_metrics, no direct documentation found
// Parent: ticket_metrics
// Notes: resource type: Embedded
// business_calendar - helper structure showing time in minutes for both calendar and business days
type business_calendar struct {
	Calendar int64 `json:"calendar"`
	Business int64 `json:"business"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/ticket_audits#content
// Parent: tickets
// Notes: resource type: Data
// audit - ticket changelog
type Audit struct {
	Id         int64                  `json:"id"`
	Ticket_id  int64                  `json:"ticket_id"`
	Created_at utime  	          `json:"created_at"`
	Author_id  int64                  `json:"author_id"`
	Events     []Event                `json:"events"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/ticket_audits#audit-events
// Parent: audit
// Notes: resource type: Data; Currently only `Create` and `Change` events are supported, this Union represents both
// event - change metadata
type Event struct {
	Id             int64  `json:"id"`
	Type           string `json:"type"`
	Field_name     string `json:"field_name"`
	Value          interface{} `json:"value"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/satisfaction_ratings
// Parent: ticket
// Notes: resource type: Embedded
// satisfaction_rating - survey response data
type Satisfaction_rating struct {
	Id           int64      `json:"id"`
	URL          string     `json:"url"`
	Assignee_id  int64      `json:"assignee_id"`
	Group_id     int64      `json:"group_id"`
	Requester_id int64      `json:"requester_id"`
	Ticket_id    int64      `json:"ticket_id"`
	Score        string     `json:"score"`
	Created_at   *utime `json:"created_at"`
	Updated_at   *utime `json:"updated_at"`
	Comment      string     `json:"comment"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/users
// Parent: root
// Notes: resource type: Data
// user - basic user object
type User struct {
	Id                    int64             `json:"id"`
	Email                 string            `json:"email"`
	Name                  string            `json:"name"`
	Active                bool              `json:"active"`
	Alias                 string            `json:"alias"`
	Created_at            *utime        `json:"created_at"`
	Details               string            `json:"details"`
	External_id           string            `json:"exteranl_id"`
	Last_login_at         *utime        `json:"last_login_at"`
	Locale                string            `json:"locale"`
	Locale_id             int64             `json:"locale_id"`
	Moderator             bool              `json:"moderator"`
	Notes                 string            `json:"Notes"`
	Only_private_comments bool              `json:"only_private_comments"`
	Organization_id       int64             `json:"organization_id"`
	Default_group_id      int64             `json:"default_group_id"`
	Phone                 string            `json:"phone"`
	Photo                 *attachment       `json:"photo"`
	Restricted_agent      bool              `json:"restricted_agent"`
	Role                  string            `json:"role"`
	Shared                bool              `json:"shared"`
	Shared_agent          bool              `json:"shared_agent"`
	Signature             string            `json:"signature"`
	Suspended             bool              `json:"suspended"`
	Tags                  []string          `json:"tags"`
	Ticket_restrcition    string            `json:"ticket_restriction"`
	Time_zone             string            `json:"time_zone"`
	Two_factor_enabled    bool              `json:"two_factor_auth_enabled"`
	Updated_at            utime      	`json:"updated_at"`
	URL                   string            `json:"url"`
	User_fields           map[string]string `json:"user_fields"`
	Verified              bool              `json:"verified"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/user_fields
// Parent:  user
// Notes: resource type: Metadata
// user - customized fields for user object
type user_field struct {
	Id                    int64                  `json:"id"`
	URL                   string                 `json:"url"`
	Key                   string                 `json:"key"`
	Type                  string                 `json:"type"`
	Title                 string                 `json:"title"`
	Raw_title             string                 `json:"raw_title"`
	Description           string                 `json:"description"`
	Raw_Description       string                 `json:"raw_description"`
	Position              int64                  `json:"position"`
	Active                bool                   `json:"active"`
	System                bool                   `json:"system"`
	Regexp_for_validation string                 `json:"regexp_for_validation"`
	Created_at            utime             `json:"created_at"`
	Updated_at            utime             `json:"updated_at"`
	Tag                   string                 `json:"tag"`
	Custom_field_options  map[string]interface{} `json:"custom_filed_options"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/organizations
// Parent: root
// Notes: resource type: Data
// organization - basic organization object
type Organization struct {
	Id         int64      `json:"id"`
	URL        string     `json:"url"`
	Name       string     `json:"name"`
	Created_at utime   `json:"created_at"`
	Updated_at utime 	`json:"updated_at"`
	Group_id   int64      `json:"group_id"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/organization_fields
// Parent: organization
// Notes: resource type: metadata
// organization_filed - custom organization attributes
type organization_field struct {
	Id                    int64                  `json:"id"`
	URL                   string                 `json:"url"`
	Key                   string                 `json:"key"`
	Type                  string                 `json:"type"`
	Title                 string                 `json:"title"`
	Raw_title             string                 `json:"raw_title"`
	Description           string                 `json:"description"`
	Raw_description       string                 `json:"raw_description"`
	Position              int64                  `json:"position"`
	Active                bool                   `json:"active"`
	System                bool                   `json:"system"`
	Regexp_for_validation string                 `json:"regexp_for_validation"`
	Created_at            utime             `json:"created_at"`
	Updated_at            utime             `json:"updated_at"`
	Tag                   string                 `json:"tag"`
	Custom_field_options  map[string]interface{} `json:"custom_field_options"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/groups
// Parent: root
// Notes: resource type: Data
// group - ticket working group
type Group struct {
	Id         int64      `json:"id"`
	Name       string     `json:"name"`
	Created_at utime `json:"created_at"`
	Updated_at utime `json:"updated_at"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/attachments
// Parent: root(common)
// Notes: resource type: Embedded; shared across various objects
// attachment - attachment metadata
type attachment struct {
	Id           int64   `json:"id"`
	File_name    string  `json:"file_name"`
	Content_url  string  `json:"content_url"`
	Content_type string  `json:"content_type"`
	Size         int64   `json:"size"`
	Thumbnails   []photo `json:"thumbnails"`
	Inline       bool    `json:"inline"`
}

// Doc: derived from example in ticket_metrics, no direct documentation found
// Parent: attachment
// Notes: resource type: Embedded; shared across various objects
// photo - thumbnail metadata
type photo struct {
	Id           int64  `json:"id"`
	File_name    string `json:"file_name"`
	Content_url  string `json:"content_url"`
	Content_type string `json:"content_type"`
	Size         int64  `json:"size"`
}

// Doc: https://developer.zendesk.com/rest_api/docs/core/ticket_audits.html#the-via-object
// Parent: root(common)
// Notes: resource type: Embedded; shared across various objects
// via: object creation metadata
type via struct {
	Channel string                 `json:"channel"`
	Source  map[string]interface{} `json:"source"`
}
