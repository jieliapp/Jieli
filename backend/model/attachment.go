package model

import "github.com/burugo/thing"

type Attachment struct {
	thing.BaseModel
	UserID        int64  `json:"user_id" db:"user_id,index"`
	PublicID      string `json:"public_id" db:"public_id,index"`
	MediaType     string `json:"media_type" db:"media_type"`
	Extension     string `json:"extension" db:"extension"`
	StorageDriver string `json:"storage_driver" db:"storage_driver"`
	StorageKey    string `json:"storage_key" db:"storage_key,index"`
	SizeBytes     int    `json:"size_bytes" db:"size_bytes,default:0"`
}

var AttachmentDB *thing.Thing[*Attachment]

func (Attachment) TableName() string {
	return "attachments"
}

type ThreadAttachment struct {
	thing.BaseModel
	UserID        int64  `json:"user_id" db:"user_id,index"`
	ThreadID      int64  `json:"thread_id" db:"thread_id,index,unique:uniq_thread_attachments_thread_attachment"`
	AttachmentID  int64  `json:"attachment_id" db:"attachment_id,index,unique:uniq_thread_attachments_thread_attachment"`
	PublicID      string `json:"public_id" db:"public_id,index"`
	StorageDriver string `json:"storage_driver" db:"storage_driver"`
	StorageKey    string `json:"storage_key" db:"storage_key,index"`
}

var ThreadAttachmentDB *thing.Thing[*ThreadAttachment]

func (ThreadAttachment) TableName() string {
	return "thread_attachments"
}
