package profile

import (
	"fmt"
	"time"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// Validate 返回全部结构问题（不短路）。
func (f *File) Validate() []error {
	var errsOut []error
	if f.APIVersion != APIVersionV1 {
		errsOut = append(errsOut, &errs.Invalid{Field: FieldAPIVersion, Value: f.APIVersion,
			Reason: "expect " + APIVersionV1})
	}
	if f.Kind != KindPipeline {
		errsOut = append(errsOut, &errs.Invalid{Field: FieldKind, Value: f.Kind,
			Reason: "expect " + KindPipeline})
	}
	if f.Metadata.Name == "" {
		errsOut = append(errsOut, &errs.Missing{What: FieldMetadata})
	}
	if f.Spec.Project.Platform == "" {
		errsOut = append(errsOut, &errs.Missing{What: FieldProject + ".platform"})
	}
	if len(f.Spec.Stages) == 0 {
		errsOut = append(errsOut, &errs.Missing{What: FieldStages})
	}
	seen := map[string]bool{}
	for i, st := range f.Spec.Stages {
		path := fmt.Sprintf("%s[%d]", FieldStages, i)
		if st.Name == "" {
			errsOut = append(errsOut, &errs.Missing{What: path + ".name"})
		} else if seen[st.Name] {
			errsOut = append(errsOut, &errs.Invalid{Field: path + ".name", Value: st.Name, Reason: "duplicate stage name"})
		}
		seen[st.Name] = true
		if len(st.Run) == 0 {
			errsOut = append(errsOut, &errs.Missing{What: path + ".run"})
		}
		for j, arg := range st.Run {
			if arg == "" {
				errsOut = append(errsOut, &errs.Missing{What: fmt.Sprintf("%s.run[%d]", path, j)})
			}
		}
		if st.Workdir == "" {
			errsOut = append(errsOut, &errs.Missing{What: path + ".workdir"})
		}
		if st.Timeout != "" {
			if _, err := time.ParseDuration(st.Timeout); err != nil {
				errsOut = append(errsOut, &errs.Invalid{Field: path + ".timeout", Value: st.Timeout, Reason: "expect duration like 10m"})
			}
		}
	}
	for i, dc := range f.Spec.Checks.Dockerfiles {
		if dc.Path == "" {
			errsOut = append(errsOut, &errs.Missing{What: fmt.Sprintf("spec.checks.dockerfiles[%d].path", i)})
		}
	}
	return errsOut
}
