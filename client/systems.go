// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2020-2023 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package client

import (
	"bytes"
	"encoding/json"
	"fmt"

	"golang.org/x/xerrors"

	"github.com/snapcore/snapd/gadget"
	"github.com/snapcore/snapd/gadget/device"
	"github.com/snapcore/snapd/secboot"
	"github.com/snapcore/snapd/snap"
)

// SystemModelData contains information about the model
type SystemModelData struct {
	// Model as the model assertion
	Model string `json:"model,omitempty"`
	// BrandID corresponds to brand-id in the model assertion
	BrandID string `json:"brand-id,omitempty"`
	// DisplayName is human friendly name, corresponds to display-name in
	// the model assertion
	DisplayName string `json:"display-name,omitempty"`
}

type System struct {
	// Current is true when the system running now was installed from that
	// recovery seed
	Current bool `json:"current,omitempty"`
	// Label of the recovery system
	Label string `json:"label,omitempty"`
	// Model information
	Model SystemModelData `json:"model,omitempty"`
	// Brand information
	Brand snap.StoreAccount `json:"brand,omitempty"`
	// Actions available for this system
	Actions []SystemAction `json:"actions,omitempty"`
	// DefaultRecoverySystem is true when the system is the default recovery system
	DefaultRecoverySystem bool `json:"default-recovery-system,omitempty"`
}

type SystemAction struct {
	// Title is a user presentable action description
	Title string `json:"title,omitempty"`
	// Mode given action can be executed in
	Mode string `json:"mode,omitempty"`
}

// ListSystems list all systems available for seeding or recovery.
func (client *Client) ListSystems() ([]System, error) {
	type systemsResponse struct {
		Systems []System `json:"systems,omitempty"`
	}

	var rsp systemsResponse

	if _, err := client.doSync("GET", "/v2/systems", nil, nil, nil, &rsp); err != nil {
		return nil, xerrors.Errorf("cannot list recovery systems: %v", err)
	}
	return rsp.Systems, nil
}

// DoSystemAction issues a request to perform an action using the given seed
// system and its mode.
func (client *Client) DoSystemAction(systemLabel string, action *SystemAction) error {
	if systemLabel == "" {
		return fmt.Errorf("cannot request an action without the system")
	}
	if action == nil {
		return fmt.Errorf("cannot request an action without one")
	}
	// deeper verification is done by the backend

	req := struct {
		Action string `json:"action"`
		*SystemAction
	}{
		Action:       "do",
		SystemAction: action,
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(&req); err != nil {
		return err
	}
	if _, err := client.doSync("POST", "/v2/systems/"+systemLabel, nil, nil, &body, nil); err != nil {
		return xerrors.Errorf("cannot request system action: %v", err)
	}
	return nil
}

// RebootToSystem issues a request to reboot into system with the
// given label and the given mode.
//
// When called without a systemLabel and without a mode it will just
// trigger a regular reboot.
//
// When called without a systemLabel but with a mode it will use
// the current system to enter the given mode.
//
// Note that "recover" and "run" modes are only available for the
// current system.
func (client *Client) RebootToSystem(systemLabel, mode string) error {
	// verification is done by the backend

	req := struct {
		Action string `json:"action"`
		Mode   string `json:"mode"`
	}{
		Action: "reboot",
		Mode:   mode,
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(&req); err != nil {
		return err
	}
	if _, err := client.doSync("POST", "/v2/systems/"+systemLabel, nil, nil, &body, nil); err != nil {
		if systemLabel != "" {
			return xerrors.Errorf("cannot request system reboot into %q: %v", systemLabel, err)
		}
		return xerrors.Errorf("cannot request system reboot: %v", err)
	}
	return nil
}

type StorageEncryptionSupport string

const (
	// forcefully disabled by the device
	StorageEncryptionSupportDisabled = "disabled"
	// encryption available and usable
	StorageEncryptionSupportAvailable = "available"
	// encryption unavailable but not required
	StorageEncryptionSupportUnavailable = "unavailable"
	// encryption unavailable and required, this is an error
	StorageEncryptionSupportDefective = "defective"
)

type StorageEncryptionFeature string

const (
	// Indicates that passphrase authentication is available.
	StorageEncryptionFeaturePassphraseAuth StorageEncryptionFeature = "passphrase-auth"
	// Indicates that PIN authentication is available.
	StorageEncryptionFeaturePINAuth StorageEncryptionFeature = "pin-auth"
)

type StorageEncryption struct {
	// Support describes the level of hardware support available.
	Support StorageEncryptionSupport `json:"support"`

	// Features is a list of available encryption features.
	Features []StorageEncryptionFeature `json:"features"`

	// StorageSafety can have values of asserts.StorageSafety
	StorageSafety string `json:"storage-safety,omitempty"`

	// Type has values of device.EncryptionType.
	Type device.EncryptionType `json:"encryption-type,omitempty"`

	// UnavailableReason describes why the encryption is not
	// available in a human readable form. Depending on if
	// encryption is required or not this should be presented to
	// the user as either an error or as information.
	UnavailableReason string `json:"unavailable-reason,omitempty"`

	// AvailabilityCheckErrors reports errors detected during preinstall check.
	AvailabilityCheckErrors []secboot.PreinstallErrorDetails `json:"availability-check-errors,omitempty"`
}

type SystemDetails struct {
	// First part is designed to look like `client.System` - the
	// only difference is how the model is represented
	Current bool              `json:"current,omitempty"`
	Label   string            `json:"label,omitempty"`
	Model   map[string]any    `json:"model,omitempty"`
	Brand   snap.StoreAccount `json:"brand,omitempty"`
	Actions []SystemAction    `json:"actions,omitempty"`

	// Volumes contains the volumes defined from the gadget snap
	Volumes map[string]*gadget.Volume `json:"volumes,omitempty"`

	StorageEncryption *StorageEncryption `json:"storage-encryption,omitempty"`

	// AvailableOptional contains the optional snaps and components that are
	// available in this system.
	AvailableOptional AvailableForInstall `json:"available-optional"`
}

// AvailableForInstall contains information about snaps and components that are
// optional in the system's model, but are available for installation.
type AvailableForInstall struct {
	// Snaps contains the names of optional snaps that are available for installation.
	Snaps []string `json:"snaps,omitempty"`
	// Components contains a mapping of snap names to lists of the names of
	// optional components that are available for installation.
	Components map[string][]string `json:"components,omitempty"`
}

func (client *Client) SystemDetails(systemLabel string) (*SystemDetails, error) {
	var rsp SystemDetails

	if _, err := client.doSync("GET", "/v2/systems/"+systemLabel, nil, nil, nil, &rsp); err != nil {
		return nil, xerrors.Errorf("cannot get details for system %q: %v", systemLabel, err)
	}
	gadget.SetEnclosingVolumeInStructs(rsp.Volumes)
	return &rsp, nil
}

type InstallStep string

const (
	// Creates a change to setup encryption for the partitions
	// with system-{data,save} roles. The successful change has a
	// created device mapper devices ready to use.
	InstallStepSetupStorageEncryption InstallStep = "setup-storage-encryption"

	// Generates a recovery key to be used in the "finish" step.
	// InstallStepSetupStorageEncryption must be called before calling
	// this step.
	InstallStepGenerateRecoveryKey InstallStep = "generate-recovery-key"

	// Creates a change to finish the installation. The change
	// ensures all volume structure content is written to disk, it
	// sets up boot, kernel etc and when finished the installed
	// system is ready for reboot.
	InstallStepFinish InstallStep = "finish"
)

type InstallSystemOptions struct {
	// Step is the install step, either "setup-storage-encryption"
	// or "finish".
	Step InstallStep `json:"step,omitempty"`

	// OnVolumes is the volume description of the volumes that the
	// given step should operate on.
	OnVolumes map[string]*gadget.Volume `json:"on-volumes,omitempty"`
	// OptionalInstall contains the optional snaps and components that should be
	// installed on the system. Omitting this field will result in all optional
	// snaps and components being installed. To install none of the optional
	// snaps and components, provide an empty OptionalInstallRequest with the
	// All field set to false.
	OptionalInstall *OptionalInstallRequest `json:"optional-install,omitempty"`
	// VolumesAuth contains options for volumes authentication (e.g. passphrase
	// authentication). If VolumesAuth is nil, the default is to have no
	// authentication.
	VolumesAuth *device.VolumesAuthOptions `json:"volumes-auth,omitempty"`
}

type OptionalInstallRequest struct {
	AvailableForInstall
	// All is true if all optional snaps and components should be installed. It
	// is invalid to set both All and the individual fields in AvailableForInstall.
	All bool `json:"all,omitempty"`
}

// InstallSystem will perform the given install step for the given volumes
func (client *Client) InstallSystem(systemLabel string, opts *InstallSystemOptions) (changeID string, err error) {
	if systemLabel == "" {
		return "", fmt.Errorf("cannot install with an empty system label")
	}

	// verification is done by the backend
	req := struct {
		Action string `json:"action"`
		*InstallSystemOptions
	}{
		Action:               "install",
		InstallSystemOptions: opts,
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(&req); err != nil {
		return "", err
	}
	chgID, err := client.doAsync("POST", "/v2/systems/"+systemLabel, nil, nil, &body)
	if err != nil {
		return "", xerrors.Errorf("cannot request system install for %q: %v", systemLabel, err)
	}
	return chgID, nil
}

// GeneratePreInstallRecoveryKey generates a recovery key to be enrolled in
// the finish step `InstallStepFinish`.
//
// Note: `InstallStepSetupStorageEncryption` must be called before recovery
// key generation.
func (client *Client) GeneratePreInstallRecoveryKey(systemLabel string) (recoveryKey string, err error) {
	if systemLabel == "" {
		return "", fmt.Errorf("cannot generate recovery key with an empty system label")
	}

	req := struct {
		Action string `json:"action"`
		*InstallSystemOptions
	}{
		Action: "install",
		InstallSystemOptions: &InstallSystemOptions{
			Step: "generate-recovery-key",
		},
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(&req); err != nil {
		return "", err
	}

	var rsp struct {
		RecoveryKey string `json:"recovery-key"`
	}
	if _, err := client.doSync("POST", "/v2/systems/"+systemLabel, nil, nil, &body, &rsp); err != nil {
		return "", xerrors.Errorf("cannot generate recovery key for system %q: %v", systemLabel, err)
	}
	return rsp.RecoveryKey, nil
}

// CreateSystemOptions contains the options for creating a new recovery system.
type CreateSystemOptions struct {
	// Label is the label of the new system.
	Label string `json:"label,omitempty"`
	// ValidationSets is a list of validation sets that snaps in the newly
	// created system should be validated against.
	ValidationSets []string `json:"validation-sets,omitempty"`
	// TestSystem is true if the system should be tested by rebooting into the
	// new system.
	TestSystem bool `json:"test-system,omitempty"`
	// MarkDefault is true if the system should be marked as the default
	// recovery system.
	MarkDefault bool `json:"mark-default,omitempty"`
	// Offline is true if the system should be created without reaching out to
	// the store. In the JSON variant of the API, only pre-installed
	// snaps/assertions will be considered.
	Offline bool `json:"offline,omitempty"`
}

// QualityCheckOptions contains the passphrase or PIN whose quality should be checked.
type QualityCheckOptions struct {
	Passphrase string `json:"passphrase,omitempty"`
	PIN        string `json:"pin,omitempty"`
}
