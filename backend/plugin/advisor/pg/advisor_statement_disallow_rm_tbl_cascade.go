package pg

// Framework code is generated by the generator.

import (
	"encoding/json"

	pgquery "github.com/pganalyze/pg_query_go/v5"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/plugin/advisor"
	storepb "github.com/bytebase/bytebase/proto/generated-go/store"
)

var (
	_ advisor.Advisor = (*StatementDisallowOnDelCascadeAdvisor)(nil)
)

func init() {
	advisor.Register(storepb.Engine_POSTGRES, advisor.PostgreSQLStatementDisallowRemoveTblCascade, &StatementDisallowRemoveTblCascadeAdvisor{})
}

// StatementDisallowRemoveTblCascadeAdvisor is the advisor checking the disallow cascade.
type StatementDisallowRemoveTblCascadeAdvisor struct {
}

// Check checks for DML dry run.
func (*StatementDisallowRemoveTblCascadeAdvisor) Check(ctx advisor.Context, _ string) ([]advisor.Advice, error) {
	stmt := ctx.Statements
	if stmt == "" {
		return []advisor.Advice{
			{
				Status:  advisor.Success,
				Code:    advisor.Ok,
				Title:   "OK",
				Content: "",
			},
		}, nil
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
	if err != nil {
		return nil, err
	}

	jsonText, err := pgquery.ParseToJSON(stmt)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse statement to JSON")
	}

	var jsonData map[string]any
	if err := json.Unmarshal([]byte(jsonText), &jsonData); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON")
	}

	cascadeLocations := cascadeNumRecursive(jsonData, 0, isDropCascade)
	cascadePositions := convertLocationsToPositions(stmt, cascadeLocations)

	var adviceList []advisor.Advice
	for _, p := range cascadePositions {
		adviceList = append(adviceList, advisor.Advice{
			Status:  level,
			Title:   string(ctx.Rule.Type),
			Content: "The use of CASCADE is not permitted when removing a table",
			Code:    advisor.StatementDisallowCascade,
			Line:    p.line + 1,
			Column:  p.column + 1,
		})
	}
	if len(adviceList) == 0 {
		adviceList = append(adviceList, advisor.Advice{
			Status:  advisor.Success,
			Code:    advisor.Ok,
			Title:   "OK",
			Content: "",
		})
	}
	return adviceList, nil
}

type pos struct {
	line   int
	column int
}

func convertLocationsToPositions(statement string, locations []int) []pos {
	idx := 0
	line := 0
	columnStart := 0
	var positions []pos
	for i, c := range statement {
		if c == '\n' {
			line++
			columnStart = i + 1
			continue
		}
		if idx < len(locations) && i >= locations[idx] {
			positions = append(positions, pos{line, i - columnStart})
			idx++
		}
	}
	for idx < len(locations) {
		positions = append(positions, pos{line, 0})
		idx++
	}
	return positions
}

func cascadeNumRecursive(jsonData map[string]any, stmtLocation int, checkCondition func(map[string]any) bool) []int {
	if l, ok := jsonData["stmt_location"]; ok {
		if l, ok := l.(float64); ok {
			stmtLocation = int(l)
		}
	}

	var cascadeLocations []int

	for _, value := range jsonData {
		switch value := value.(type) {
		case map[string]any:
			cascadeLocations = append(cascadeLocations, cascadeNumRecursive(value, stmtLocation, checkCondition)...)
		case []any:
			for _, v := range value {
				mv, ok := v.(map[string]any)
				if !ok {
					continue
				}
				cascadeLocations = append(cascadeLocations, cascadeNumRecursive(mv, stmtLocation, checkCondition)...)
			}
		}
	}

	if checkCondition(jsonData) {
		cascadeLocations = append(cascadeLocations, stmtLocation)
	}

	return cascadeLocations
}

func isDropCascade(json map[string]any) bool {
	return json["behavior"] == "DROP_CASCADE"
}