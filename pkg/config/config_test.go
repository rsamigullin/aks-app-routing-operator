// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var configTestCases = []struct {
	Name  string
	Conf  *Config
	Error string
}{
	{
		Name: "valid-minimal",
		Conf: &Config{
			NS:          "test-namespace",
			Registry:    "test-registry",
			MSIClientID: "test-msi-client-id",
			TenantID:    "test-tenant-id",
			Cloud:       "test-cloud",
			Location:    "test-location",
		},
	},
	{
		Name: "valid-full",
		Conf: &Config{
			NS:            "test-namespace",
			Registry:      "test-registry",
			MSIClientID:   "test-msi-client-id",
			TenantID:      "test-tenant-id",
			Cloud:         "test-cloud",
			Location:      "test-location",
			DNSZoneRG:     "test-dns-zone-rg",
			DNSZoneSub:    "test-dns-zone-sub",
			DNSZoneDomain: "test-dns-zone-domain",
		},
	},
	{
		Name: "missing-namespace",
		Conf: &Config{
			Registry:    "test-registry",
			MSIClientID: "test-msi-client-id",
			TenantID:    "test-tenant-id",
			Cloud:       "test-cloud",
			Location:    "test-location",
		},
		Error: "--namespace is required",
	},
	{
		Name: "missing-registry",
		Conf: &Config{
			NS:          "test-namespace",
			MSIClientID: "test-msi-client-id",
			TenantID:    "test-tenant-id",
			Cloud:       "test-cloud",
			Location:    "test-location",
		},
		Error: "--registry is required",
	},
	{
		Name: "missing-msi",
		Conf: &Config{
			NS:       "test-namespace",
			Registry: "test-registry",
			TenantID: "test-tenant-id",
			Cloud:    "test-cloud",
			Location: "test-location",
		},
		Error: "--msi is required",
	},
	{
		Name: "missing-tenant-id",
		Conf: &Config{
			NS:          "test-namespace",
			Registry:    "test-registry",
			MSIClientID: "test-msi-client-id",
			Cloud:       "test-cloud",
			Location:    "test-location",
		},
		Error: "--tenant-id is required",
	},
	{
		Name: "missing-cloud",
		Conf: &Config{
			NS:          "test-namespace",
			Registry:    "test-registry",
			MSIClientID: "test-msi-client-id",
			TenantID:    "test-tenant-id",
			Location:    "test-location",
		},
		Error: "--cloud is required",
	},
	{
		Name: "missing-location",
		Conf: &Config{
			NS:          "test-namespace",
			Registry:    "test-registry",
			MSIClientID: "test-msi-client-id",
			TenantID:    "test-tenant-id",
			Cloud:       "test-cloud",
		},
		Error: "--location is required",
	},
	{
		Name: "missing-dns-zone-rg",
		Conf: &Config{
			NS:            "test-namespace",
			Registry:      "test-registry",
			MSIClientID:   "test-msi-client-id",
			TenantID:      "test-tenant-id",
			Cloud:         "test-cloud",
			Location:      "test-location",
			DNSZoneSub:    "test-dns-zone-sub",
			DNSZoneDomain: "test-dns-zone-domain",
		},
		Error: "--dns-zone-resource-group is required",
	},
	{
		Name: "missing-dns-zone-sub",
		Conf: &Config{
			NS:            "test-namespace",
			Registry:      "test-registry",
			MSIClientID:   "test-msi-client-id",
			TenantID:      "test-tenant-id",
			Cloud:         "test-cloud",
			Location:      "test-location",
			DNSZoneRG:     "test-dns-zone-rg",
			DNSZoneDomain: "test-dns-zone-domain",
		},
		Error: "--dns-zone-subscription is required",
	},
	{
		Name: "missing-dns-zone-domain",
		Conf: &Config{
			NS:          "test-namespace",
			Registry:    "test-registry",
			MSIClientID: "test-msi-client-id",
			TenantID:    "test-tenant-id",
			Cloud:       "test-cloud",
			Location:    "test-location",
			DNSZoneRG:   "test-dns-zone-rg",
			DNSZoneSub:  "test-dns-zone-sub",
		},
		Error: "--dns-zone-domain is required",
	},
}

func TestConfigValidate(t *testing.T) {
	for _, tc := range configTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.Conf.Validate()
			if tc.Error == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.Error)
			}
		})
	}
}
