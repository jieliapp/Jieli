package model

import "github.com/burugo/thing"

// EmailToken is a time-limited, single-use proof of email ownership.
type EmailToken struct {
	thing.BaseModel
	TokenHash    string `db:"token_hash,unique" json:"-"`
	Email        string `db:"email,index"`
	Purpose      string `db:"purpose"`
	DisplayName  string `db:"display_name,default:''"`
	PasswordHash string `db:"password_hash" json:"-"`
	ExpiresAtMS  int64  `db:"expires_at_ms"`
	Used         bool   `db:"used"`
}

func (EmailToken) TableName() string { return "email_tokens" }
