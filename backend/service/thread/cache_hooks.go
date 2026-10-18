package thread

import (
	"context"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/burugo/thing"
)

func init() {
	for _, eventType := range []thing.EventType{
		thing.EventTypeAfterSave,
		thing.EventTypeAfterDelete,
		thing.EventTypeAfterSoftDelete,
	} {
		thing.RegisterListener(eventType, invalidateThreadServiceCaches)
	}
}

func invalidateThreadServiceCaches(_ context.Context, _ thing.EventType, row interface{}, _ interface{}) error {
	switch typed := row.(type) {
	case *model.Thread:
		invalidateThreadMessagesCache(typed.ID)
		invalidateUserFacets(typed.UserID)
	case *model.ThreadMessage:
		invalidateThreadMessagesCache(typed.ThreadID)
	case *model.ThreadLabel:
		invalidateUserFacets(typed.UserID)
	}
	return nil
}
