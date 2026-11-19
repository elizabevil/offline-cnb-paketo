// Package manifest validates the lock file structure.
package manifest

import (
	"errors"
	"fmt"
	"strings"
)

// Validate returns every structural problem found in the manifest.
// It performs no I/O: callers decide how to print or count the findings.
func (f *File) Validate() []error {
	var errs []error
	errs = append(errs, f.storageErrors()...)
	errs = append(errs, f.metaErrors()...)
	errs = append(errs, f.assetErrors()...)
	return errs
}

// storageErrors checks schema and storage layout.
func (f *File) storageErrors() []error {
	var errs []error
	switch f.Bundle.Layout {
	case "", LayoutArchFirst:
	default:
		// kind-first is reserved but not implemented: bundle paths assume
		// <platform>/archives|binaries.
		errs = append(errs, fmt.Errorf("[bundle].layout only supports %q for now, got %q",
			LayoutArchFirst, f.Bundle.Layout))
	}
	if f.Schema != 1 {
		errs = append(errs, fmt.Errorf("schema must be 1, got %d", f.Schema))
	}
	return errs
}

// metaErrors checks the declared platform matrix.
func (f *File) metaErrors() []error {
	var errs []error
	for _, p := range f.Platforms() {
		if !KnownPlatforms[p] {
			errs = append(errs, fmt.Errorf("meta.platforms contains unknown platform %s", p))
		}
	}
	return errs
}

// assetErrors checks ids, versions, unpack rules and platform sources.
func (f *File) assetErrors() []error {
	var errs []error
	seen := map[string]bool{}
	byID := map[string]map[string]string{} // id -> unpack dir -> version
	for _, a := range f.Assets {
		errs = append(errs, f.assetIdentityErrors(a, seen)...)
		errs = append(errs, a.unpackErrors()...)
		errs = append(errs, f.platformPresenceErrors(a)...)
		errs = append(errs, a.multiArchErrors()...)
		errs = append(errs, a.sourceErrors()...)
		errs = append(errs, dirConflictError(a, byID)...)
		errs = append(errs, a.urlErrors()...)
	}
	return errs
}

// assetIdentityErrors checks id/version presence and uniqueness.
func (f *File) assetIdentityErrors(a Asset, seen map[string]bool) []error {
	if a.ID == "" {
		return []error{errors.New("asset without id")}
	}
	var errs []error
	if seen[a.Ref()] {
		errs = append(errs, errors.New("duplicate material version: "+string(a.Ref())))
	}
	seen[a.Ref()] = true
	if a.Version == "" {
		errs = append(errs, errors.New(a.ID+": missing version"))
	}
	return errs
}

// unpackErrors checks unpack.kind, strip and expect.
func (a Asset) unpackErrors() []error {
	var errs []error
	switch a.Unpack.Kind {
	case KindTGZ, KindTarGz, KindZip, KindBin:
	default:
		errs = append(errs, fmt.Errorf("%s: unpack.kind must be one of tgz / tar.gz / zip / bin, got %q",
			a.ID, a.Unpack.Kind))
	}
	if a.Unpack.Strip < 0 {
		errs = append(errs, fmt.Errorf("%s: unpack.strip must not be negative (%d)", a.ID, a.Unpack.Strip))
	}
	if len(a.Unpack.Expect) == 0 {
		errs = append(errs, errors.New(a.ID+": unpack.expect must declare at least one artifact"))
	}
	return errs
}

// platformPresenceErrors checks that declared platforms carry source tables.
func (f *File) platformPresenceErrors(a Asset) []error {
	var errs []error
	if len(a.Platform) == 0 {
		errs = append(errs, errors.New(a.ID+": missing [asset.platform.<platform>] table"))
	}
	for _, want := range f.Meta.Platforms {
		if _, ok := a.Platform[want]; !ok {
			errs = append(errs, fmt.Errorf("%s: missing platform %s declared by meta.platforms", a.ID, want))
		}
	}
	return errs
}

// multiArchErrors requires {arch} in expect for multi-arch materials.
func (a Asset) multiArchErrors() []error {
	if !a.MultiArch {
		return nil
	}
	for _, e := range a.Unpack.Expect {
		if strings.Contains(e, "{arch}") {
			return nil
		}
	}
	return []error{errors.New(a.ID + ": multi_arch materials must declare a {arch} artifact in unpack.expect")}
}

// sourceErrors checks each platform source kind and its required field.
func (a Asset) sourceErrors() []error {
	var errs []error
	for name, src := range a.Platform {
		switch src.Kind {
		case "", KindHTTP:
			if src.URL == "" {
				errs = append(errs, fmt.Errorf("%s: %s is missing url", a.ID, name))
			}
		case KindLocal:
			if src.Source == "" {
				errs = append(errs, fmt.Errorf("%s: %s is missing source", a.ID, name))
			}
		default:
			errs = append(errs, fmt.Errorf("%s: %s has unknown kind %q", a.ID, name, src.Kind))
		}
	}
	return errs
}

// dirConflictError reports two versions sharing one unpack directory.
func dirConflictError(a Asset, byID map[string]map[string]string) []error {
	if byID[a.ID] == nil {
		byID[a.ID] = map[string]string{}
	}
	dir := a.Dir()
	if prev, ok := byID[a.ID][dir]; ok && prev != a.Version {
		return []error{fmt.Errorf(
			"%s %s and %s share unpack directory %q and would overwrite each other; set a distinct unpack.dir",
			a.ID, a.Version, prev, dir)}
	}
	byID[a.ID][dir] = a.Version
	return nil
}

// urlErrors checks the platform whitelist and rendered URL scheme.
func (a Asset) urlErrors() []error {
	var errs []error
	for p, src := range a.Platform {
		if !KnownPlatforms[p] {
			errs = append(errs, fmt.Errorf("%s: unsupported platform %s", a.ID, p))
			continue
		}
		if src.Kind == KindLocal {
			continue
		}
		rendered := strings.ReplaceAll(src.URL, "{version}", a.Version)
		if !strings.HasPrefix(rendered, "http://") && !strings.HasPrefix(rendered, "https://") {
			errs = append(errs, fmt.Errorf("%s[%s]: url must be an absolute http(s) URL, rendered %q", a.ID, p, rendered))
		}
	}
	return errs
}
