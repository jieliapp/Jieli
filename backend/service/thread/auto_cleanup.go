package thread

import (
	"log"
	"sort"
	"sync"

	"github.com/jieliapp/jieli/backend/model"
)

// autoCleanupTriggerRatio is the usage fraction (of quota) at which auto-cleanup
// fires. autoCleanupDeleteRatio is the fraction of quota removed per run, which
// drops usage to roughly 70% so it does not re-trigger on the next upload.
const (
	autoCleanupTriggerRatio = 0.90
	autoCleanupDeleteRatio  = 0.30
)

// cleanupLocks guards against concurrent auto-cleanup runs for the same user.
// A user who cannot acquire their lock skips this run; the next upload re-checks.
var cleanupLocks sync.Map // map[int64]*sync.Mutex

func userCleanupLock(userID int64) *sync.Mutex {
	actual, _ := cleanupLocks.LoadOrStore(userID, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

// MaybeAutoCleanup deletes a user's oldest threads when their stored message
// usage reaches the trigger threshold, but only if the user has opted in. It is
// best-effort: it runs after a successful upload and its errors are logged, not
// propagated, so cleanup never fails an upload.
func MaybeAutoCleanup(user *model.User) error {
	if user == nil || user.ID == 0 || !user.AutoCleanupEnabled {
		return nil
	}
	lock := userCleanupLock(user.ID)
	if !lock.TryLock() {
		return nil
	}
	defer lock.Unlock()

	quota := model.MessageQuotaForUser(user)
	if quota < 0 {
		return nil
	}
	total, err := model.SumThreadMessageCountByUser(user.ID)
	if err != nil {
		return err
	}
	if total < int(float64(quota)*autoCleanupTriggerRatio) {
		return nil
	}

	target := int(float64(quota) * autoCleanupDeleteRatio)
	candidates, err := oldestThreadsForCleanup(user.ID)
	if err != nil {
		return err
	}

	deleted := 0
	for _, candidate := range candidates {
		if deleted >= target {
			break
		}
		if err := SoftDeleteThread(user, candidate.ID); err != nil {
			// Stop on the first failure; remaining headroom is reclaimed on the
			// next upload. Errors are logged, never returned to the upload path.
			log.Printf("auto-cleanup: user=%d deleted=%d target=%d stopped on thread=%d: %v",
				user.ID, deleted, target, candidate.ID, err)
			return nil
		}
		deleted += candidate.MessageCount
	}
	log.Printf("auto-cleanup: user=%d before=%d target=%d deleted=%d", user.ID, total, target, deleted)
	return nil
}

// oldestThreadsForCleanup returns the user's threads ordered oldest-first. The
// primary key is UserLastInteractedAtMS (least-recently touched first); threads
// missing that value fall back to ProviderCreatedMillis, then row CreatedAt.
func oldestThreadsForCleanup(userID int64) ([]*model.Thread, error) {
	threads, err := model.ThreadDB.Where("user_id = ?", userID).All()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(threads, func(i, j int) bool {
		return cleanupOrderKey(threads[i]) < cleanupOrderKey(threads[j])
	})
	return threads, nil
}

// cleanupOrderKey produces the millisecond ordering key with fallbacks for
// threads whose provider did not supply a last-interaction timestamp.
func cleanupOrderKey(t *model.Thread) int64 {
	if t.UserLastInteractedAtMS != 0 {
		return t.UserLastInteractedAtMS
	}
	if t.ProviderCreatedMillis != 0 {
		return t.ProviderCreatedMillis
	}
	return t.CreatedAt.UnixMilli()
}
