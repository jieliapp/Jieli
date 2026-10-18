package model

import "github.com/burugo/thing"

type ThreadLabel struct {
	thing.BaseModel
	UserID   int64  `json:"user_id" db:"user_id,index,index:idx_thread_labels_user_label_thread"`
	Label    string `json:"label" db:"label,index,index:idx_thread_labels_user_label_thread,unique:uniq_thread_labels_thread_label"`
	ThreadID int64  `json:"thread_id" db:"thread_id,index,index:idx_thread_labels_user_label_thread,unique:uniq_thread_labels_thread_label"`
}

var ThreadLabelDB *thing.Thing[*ThreadLabel]

func (ThreadLabel) TableName() string {
	return "thread_labels"
}
