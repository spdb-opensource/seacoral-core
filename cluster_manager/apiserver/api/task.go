package api

import (
	"time"
)

type TaskBrief struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Action string `json:"action"`
	User   string `json:"user,omitempty"`
}

func newTaskBrief(id, action, user, state string) TaskBrief {
	return TaskBrief{
		ID:     id,
		Status: state,
		Action: action,
		User:   user,
	}
}

type Task struct {
	ID         string `json:"id"`
	User       string `json:"created_user"`
	Action     string `json:"action"`
	RelateID   string `json:"relate_id"`
	Status     string `json:"status"`
	Error      string `json:"error"`
	CreatedAt  Time   `json:"created_at"`
	FinishedAt Time   `json:"finished_at"`
}

type TasksResponse []Task

// ErrorResponse
// error common response
//
// swagger:response ErrorResponse
// in: body
type ErrorResponse struct {
	// 错误代码
	Code int `json:"code"`

	// 错误信息
	Error string `json:"msg"`
}

// TaskResponse response task ID
//
// swagger:response TaskResponse
// in: body
type TaskResponse struct {
	// 任务 ID
	ID string `json:"taskID"`
}

// ObjectResponse response object id and name
//
// swagger:response ObjectResponse
// in: body
type ObjectResponse struct {
	// 对象名称
	ID string `json:"id"`
	// 对象名称
	Name string `json:"name"`
}

// TaskObjectResponse response task ID with object's id and name
//
// swagger:response TaskObjectResponse
// in: body
type TaskObjectResponse struct {
	// 对象名称
	ObjectID string `json:"id"`
	// 对象名称
	ObjectName string `json:"name"`
	// 任务 ID
	TaskID string `json:"task_id"`
}

type Editor struct {
	User      string `json:"user"`
	Timestamp Time   `json:"timestamp"`
}

func NewEditor(user string, t time.Time) Editor {
	return Editor{
		User:      user,
		Timestamp: Time(t),
	}
}

const (
	TimeFormat = "2006-01-02 15:04:05"
)

type Time time.Time

func Now() Time {
	return Time(time.Now())
}

func (t *Time) UnmarshalJSON(data []byte) (err error) {
	now, err := time.ParseInLocation(`"`+TimeFormat+`"`, string(data), time.Local)
	*t = Time(now)
	return
}

func (t Time) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, len(TimeFormat)+2)
	b = append(b, '"')
	b = time.Time(t).AppendFormat(b, TimeFormat)
	b = append(b, '"')
	return b, nil
}

func (t Time) String() string {
	return time.Time(t).Format(TimeFormat)
}
