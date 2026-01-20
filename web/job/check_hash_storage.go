package job

import (
	"x-ui/web/service"
)

type CheckHashStorageJob struct {
	tgbotService service.Tgbot
}

func NewCheckHashStorageJob() *CheckHashStorageJob {
	return new(CheckHashStorageJob)
}

// Here Run is an interface method of the Job interface
func (j *CheckHashStorageJob) Run() {
	// Remove expired hashes from storage
	if storage := j.tgbotService.GetHashStorage(); storage != nil {
		storage.RemoveExpiredHashes()
	}
}
