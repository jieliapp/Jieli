package route

import (
	"strings"

	"github.com/jieliapp/jieli/backend/model"
)

func loadSiteSettings() (model.SiteSettingsValues, error) {
	return model.LoadSiteSettings()
}

func sitePublicBaseURL() (string, error) {
	settings, err := loadSiteSettings()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(strings.TrimSpace(settings.PublicBaseURL), "/"), nil
}
