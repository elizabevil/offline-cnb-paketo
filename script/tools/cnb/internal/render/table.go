package render

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"learn-cnb-paketo/tools/cnb/internal/profile"
)

// Table 输出对齐的人类可读摘要（tabwriter 负责列对齐）。
func Table(p *profile.Plan) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "project: %s\n", p.Project)
	fmt.Fprintf(w, "platform: %s\truntime: %s\n\n", p.Platform, p.Runtime)

	fmt.Fprintln(w, "STAGE\tKIND\tINPUT\tACTION\tOUTPUT")
	for i, st := range p.Stages {
		input, action, output := stageColumns(p, st)
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, st.Kind, input, action, output)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "ASSET\tKIND\tPLATFORMS\tSTATE")
	for _, a := range p.AssetDetails {
		state := "missing"
		if a.Present {
			state = "ok"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.Ref, a.Kind, strings.Join(a.Platforms, ","), state)
	}
	_ = w.Flush()
	return b.String()
}

// stageColumns 返回阶段的三列：输入、动作、输出。
func stageColumns(p *profile.Plan, st profile.PlanStage) (input, action, output string) {
	switch st.Kind {
	case "image":
		return shortPath(p.RepoRoot, st.Dockerfile), "docker build", st.Tag
	case "command":
		return shortPath(p.RepoRoot, st.Workdir), "run " + truncate(st.Run, 40), "-"
	default:
		return "-", st.Kind, "-"
	}
}
