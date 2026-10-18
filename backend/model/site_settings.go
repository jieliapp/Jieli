package model

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/burugo/thing"
)

const (
	SiteSettingInstanceID          = "instance_id"
	SiteSettingPublicBaseURL       = "public_base_url"
	SiteSettingSMTPHost            = "smtp.host"
	SiteSettingSMTPPort            = "smtp.port"
	SiteSettingSMTPUsername        = "smtp.username"
	SiteSettingSMTPPassword        = "smtp.password"
	SiteSettingSMTPFrom            = "smtp.from"
	SiteSettingSMTPTLSMode         = "smtp.tls_mode"
	SiteSettingSMTPTested          = "smtp.tested"
	SiteSettingRegistrationEnabled = "registration_enabled"
)

// SiteSettings stores one typed site setting per row. Secret values are kept
// in EncryptedValue rather than Value.
type SiteSettings struct {
	thing.BaseModel
	Name           string `db:"name,unique"`
	Value          string `db:"value"`
	EncryptedValue string `db:"encrypted_value" json:"-"`
	ValueType      string `db:"value_type"`
}

func (SiteSettings) TableName() string { return "site_settings" }

// SiteSettingsValues is the typed configuration used by site services.
type SiteSettingsValues struct {
	InstanceID          string
	PublicBaseURL       string
	Host                string
	Port                int
	Username            string
	PasswordEncrypted   string
	From                string
	TLSMode             string
	Tested              bool
	RegistrationEnabled bool
}

// EnsureSiteSettings creates the stable instance identity once. Other values
// are intentionally left unset until an administrator configures the site.
func EnsureSiteSettings() error {
	store, err := thing.Use[*SiteSettings]()
	if err != nil {
		return err
	}
	rows, err := store.Where("name = ?", SiteSettingInstanceID).Fetch(0, 1)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return nil
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return err
	}
	return store.Save(&SiteSettings{Name: SiteSettingInstanceID, Value: "instance_" + hex.EncodeToString(random[:]), ValueType: "string"})
}

// LoadSiteSettings reads all persisted site settings and validates their types.
func LoadSiteSettings() (SiteSettingsValues, error) {
	store, err := thing.Use[*SiteSettings]()
	if err != nil {
		return SiteSettingsValues{}, err
	}
	return loadSiteSettingsFrom(store)
}

func loadSiteSettingsFrom(store *thing.Thing[*SiteSettings]) (SiteSettingsValues, error) {
	rows, err := store.Where("id > ?", 0).Fetch(0, 100)
	if err != nil {
		return SiteSettingsValues{}, err
	}
	values := SiteSettingsValues{}
	for _, row := range rows {
		expectedType := "string"
		switch row.Name {
		case SiteSettingInstanceID:
			values.InstanceID = row.Value
		case SiteSettingPublicBaseURL:
			values.PublicBaseURL = row.Value
		case SiteSettingSMTPHost:
			values.Host = row.Value
		case SiteSettingSMTPPort:
			expectedType = "int"
			values.Port, err = strconv.Atoi(row.Value)
		case SiteSettingSMTPUsername:
			values.Username = row.Value
		case SiteSettingSMTPPassword:
			expectedType = "secret"
			values.PasswordEncrypted = row.EncryptedValue
		case SiteSettingSMTPFrom:
			values.From = row.Value
		case SiteSettingSMTPTLSMode:
			values.TLSMode = row.Value
		case SiteSettingSMTPTested:
			expectedType = "bool"
			values.Tested, err = strconv.ParseBool(row.Value)
		case SiteSettingRegistrationEnabled:
			expectedType = "bool"
			values.RegistrationEnabled, err = strconv.ParseBool(row.Value)
		default:
			continue
		}
		if err != nil || row.ValueType != expectedType ||
			(expectedType == "secret" && row.Value != "") ||
			(expectedType != "secret" && row.EncryptedValue != "") {
			return SiteSettingsValues{}, errors.New("invalid site setting value for " + row.Name)
		}
	}
	if values.InstanceID == "" {
		return SiteSettingsValues{}, errors.New("site settings instance identity is missing")
	}
	return values, nil
}

// SaveSiteSettings atomically persists the typed site configuration.
func SaveSiteSettings(ctx context.Context, values SiteSettingsValues) error {
	if values.InstanceID == "" {
		return errors.New("site settings instance identity is missing")
	}
	return thing.Transaction(ctx, func(tx *thing.TxSession) error {
		store, err := thing.UseTx[*SiteSettings](tx)
		if err != nil {
			return err
		}
		return saveSiteSettingsTo(store, values)
	})
}

// UpdateSiteSettings applies a scoped change to the latest settings within one
// transaction, so concurrent administrators cannot overwrite unrelated fields.
// The callback must not perform network I/O while holding the transaction.
func UpdateSiteSettings(ctx context.Context, update func(*SiteSettingsValues) error) error {
	return thing.Transaction(ctx, func(tx *thing.TxSession) error {
		store, err := thing.UseTx[*SiteSettings](tx)
		if err != nil {
			return err
		}
		values, err := loadSiteSettingsFrom(store)
		if err != nil {
			return err
		}
		if err = update(&values); err != nil {
			return err
		}
		return saveSiteSettingsTo(store, values)
	})
}

func saveSiteSettingsTo(store *thing.Thing[*SiteSettings], values SiteSettingsValues) error {
	items := []*SiteSettings{
		{Name: SiteSettingInstanceID, Value: values.InstanceID, ValueType: "string"},
		{Name: SiteSettingPublicBaseURL, Value: values.PublicBaseURL, ValueType: "string"},
		{Name: SiteSettingSMTPHost, Value: values.Host, ValueType: "string"},
		{Name: SiteSettingSMTPPort, Value: strconv.Itoa(values.Port), ValueType: "int"},
		{Name: SiteSettingSMTPUsername, Value: values.Username, ValueType: "string"},
		{Name: SiteSettingSMTPPassword, EncryptedValue: values.PasswordEncrypted, ValueType: "secret"},
		{Name: SiteSettingSMTPFrom, Value: values.From, ValueType: "string"},
		{Name: SiteSettingSMTPTLSMode, Value: values.TLSMode, ValueType: "string"},
		{Name: SiteSettingSMTPTested, Value: strconv.FormatBool(values.Tested), ValueType: "bool"},
		{Name: SiteSettingRegistrationEnabled, Value: strconv.FormatBool(values.RegistrationEnabled), ValueType: "bool"},
	}
	for _, item := range items {
		rows, fetchErr := store.Where("name = ?", item.Name).Fetch(0, 1)
		if fetchErr != nil {
			return fetchErr
		}
		if len(rows) > 0 {
			item.ID = rows[0].ID
		}
		if err := store.Save(item); err != nil {
			return err
		}
	}
	return nil
}
