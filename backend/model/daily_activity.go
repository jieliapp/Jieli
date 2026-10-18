package model

import "github.com/burugo/thing"

type DailyActivity struct {
	thing.BaseModel
	UserID       int64  `json:"user_id" db:"user_id,index,unique:uniq_daily_activity_user_date"`
	Date         string `json:"date" db:"date,unique:uniq_daily_activity_user_date"`
	ThreadCount  int    `json:"thread_count" db:"thread_count,default:0"`
	MessageCount int    `json:"message_count" db:"message_count,default:0"`
	LinesAdded   int    `json:"lines_added" db:"lines_added,default:0"`
	LinesRemoved int    `json:"lines_removed" db:"lines_removed,default:0"`
	LinesChanged int    `json:"lines_changed" db:"lines_changed,default:0"`
	FilesChanged int    `json:"files_changed" db:"files_changed,default:0"`
}

var DailyActivityDB *thing.Thing[*DailyActivity]

func (DailyActivity) TableName() string {
	return "daily_activity"
}
