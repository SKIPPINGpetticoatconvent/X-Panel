package service

import (
	"fmt"
	"testing"
)

func TestGeoIPLocation_GetCountryCode(t *testing.T) {
	loc := &GeoIPLocation{
		IP: "1.2.3.4",
		Location: struct {
			City        string `json:"city"`
			CountryCode string `json:"country_code"`
			CountryName string `json:"country_name"`
			Latitude    string `json:"latitude"`
			Longitude   string `json:"longitude"`
			Province    string `json:"province"`
		}{
			CountryCode: "US",
			CountryName: "United States",
			City:        "New York",
		},
	}

	if got := loc.GetCountryCode(); got != "US" {
		t.Errorf("GetCountryCode() = %q, want US", got)
	}
}

func TestGeoIPLocation_GetCountry(t *testing.T) {
	loc := &GeoIPLocation{}
	loc.Location.CountryName = "Japan"

	if got := loc.GetCountry(); got != "Japan" {
		t.Errorf("GetCountry() = %q, want Japan", got)
	}
}

func TestGeoIPLocation_GetCity(t *testing.T) {
	loc := &GeoIPLocation{}
	loc.Location.City = "Tokyo"

	if got := loc.GetCity(); got != "Tokyo" {
		t.Errorf("GetCity() = %q, want Tokyo", got)
	}
}

func TestConvertIPSBToGeoIP(t *testing.T) {
	ipsb := &IPSBLocation{
		IP:          "1.2.3.4",
		CountryCode: "JP",
		CountryName: "Japan",
		City:        "Tokyo",
		RegionName:  "Kanto",
		Latitude:    35.6762,
		Longitude:   139.6503,
	}

	result := convertIPSBToGeoIP(ipsb)

	if result.IP != "1.2.3.4" {
		t.Errorf("IP = %q, want 1.2.3.4", result.IP)
	}
	if result.GetCountryCode() != "JP" {
		t.Errorf("CountryCode = %q, want JP", result.GetCountryCode())
	}
	if result.GetCountry() != "Japan" {
		t.Errorf("CountryName = %q, want Japan", result.GetCountry())
	}
	if result.GetCity() != "Tokyo" {
		t.Errorf("City = %q, want Tokyo", result.GetCity())
	}
	if result.Location.Province != "Kanto" {
		t.Errorf("Province = %q, want Kanto", result.Location.Province)
	}
	if result.Location.Latitude != fmt.Sprintf("%.4f", 35.6762) {
		t.Errorf("Latitude = %q, want %.4f", result.Location.Latitude, 35.6762)
	}
}

func TestConvertIpapiIsToGeoIP(t *testing.T) {
	loc := &IpapiIsLocation{
		IP: "5.6.7.8",
	}
	loc.Location.CountryCode = "DE"
	loc.Location.Country = "Germany"
	loc.Location.City = "Berlin"
	loc.Location.State = "Berlin"
	loc.Location.Latitude = 52.52
	loc.Location.Longitude = 13.405

	result := convertIpapiIsToGeoIP(loc)

	if result.IP != "5.6.7.8" {
		t.Errorf("IP = %q, want 5.6.7.8", result.IP)
	}
	if result.GetCountryCode() != "DE" {
		t.Errorf("CountryCode = %q, want DE", result.GetCountryCode())
	}
	if result.GetCountry() != "Germany" {
		t.Errorf("CountryName = %q, want Germany", result.GetCountry())
	}
	if result.GetCity() != "Berlin" {
		t.Errorf("City = %q, want Berlin", result.GetCity())
	}
}

func TestNewGeoIPService(t *testing.T) {
	svc := NewGeoIPService()
	if svc == nil {
		t.Fatal("NewGeoIPService() should not return nil")
	}
	if svc.client == nil {
		t.Error("client should not be nil")
	}
}

func TestNewGeoIPServiceWithClient_Nil(t *testing.T) {
	svc := NewGeoIPServiceWithClient(nil)
	if svc == nil {
		t.Fatal("NewGeoIPServiceWithClient(nil) should not return nil")
	}
	if svc.client == nil {
		t.Error("client should be auto-created when nil is passed")
	}
}
