package main

import (
	"encoding/json"
	"log/slog"
)

const redactedSecret = "***"

func logEffectiveConfig(logger *slog.Logger, cfg *Config) {
	summary, err := redactedConfigJSON(cfg)
	if err != nil {
		logger.Warn("failed to render effective config", slog.Any("error", err))
		return
	}
	logger.Info("effective config", slog.String("config", summary))
}

func redactedConfigJSON(cfg *Config) (string, error) {
	redacted := *cfg
	redacted.PasswordValue = redactIfSet(redacted.PasswordValue)
	redacted.TOTPSecret = redactIfSet(redacted.TOTPSecret)

	redacted.Destinations = append([]Destination(nil), cfg.Destinations...)
	for i := range redacted.Destinations {
		redacted.Destinations[i].PasswordValue = redactIfSet(redacted.Destinations[i].PasswordValue)
		redacted.Destinations[i].TOTPSecret = redactIfSet(redacted.Destinations[i].TOTPSecret)
		redacted.Destinations[i].URLs = append([]DestinationURL(nil), cfg.Destinations[i].URLs...)
		for j := range redacted.Destinations[i].URLs {
			redacted.Destinations[i].URLs[j].PasswordValue = redactIfSet(redacted.Destinations[i].URLs[j].PasswordValue)
			redacted.Destinations[i].URLs[j].TOTPSecret = redactIfSet(redacted.Destinations[i].URLs[j].TOTPSecret)
		}
	}

	data, err := json.MarshalIndent(redacted, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func redactIfSet(value string) string {
	if value == "" {
		return ""
	}
	return redactedSecret
}
