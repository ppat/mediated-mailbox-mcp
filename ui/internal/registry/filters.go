package registry

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
)

// dateLayout is a day as the read API and the URL write it, a UTC date.
const dateLayout = time.DateOnly

// split turns a read's filter on one dimension into the inclusion and exclusion arrays the statements
// take, nil for no filter. The word empty becomes the empty string the statements compare, which the
// pure core admits only on a dimension that can hold a value stored empty.
func split(r lens.Request, dimension string) (in, out []string) {
	f, ok := r.Filter(dimension)
	if !ok {
		return nil, nil
	}
	values := make([]string, len(f.Values))
	for i, v := range f.Values {
		if v == lens.Empty {
			v = ""
		}
		values[i] = v
	}
	if f.Exclude {
		return nil, values
	}
	return values, nil
}

// nullable is a filter on a dimension with a null group, whose none value names that group and is
// passed to the statements apart from the stored values.
type nullable struct {
	in, out         []string
	inNone, outNone bool
}

func splitNullable(r lens.Request, dimension string) nullable {
	in, out := split(r, dimension)
	var n nullable
	for _, v := range in {
		if v == lens.None {
			n.inNone = true
		} else {
			n.in = append(n.in, v)
		}
	}
	for _, v := range out {
		if v == lens.None {
			n.outNone = true
		} else {
			n.out = append(n.out, v)
		}
	}
	return n
}

// dates are UTC dates as the statements take them. The pure core admits only calendar dates, so each
// parses.
func dates(values []string) ([]pgtype.Date, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]pgtype.Date, 0, len(values))
	for _, v := range values {
		t, err := time.Parse(dateLayout, v)
		if err != nil {
			return nil, err
		}
		out = append(out, pgtype.Date{Time: t, Valid: true})
	}
	return out, nil
}

// whole are whole numbers as the statements take them. The pure core admits only whole numbers that
// fit a stored integer, so each parses.
func whole(values []string) ([]int32, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]int32, 0, len(values))
	for _, v := range values {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return nil, err
		}
		out = append(out, int32(n))
	}
	return out, nil
}

// uuid is a nullable stored identifier, nil when null.
func uuid(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}

// RawJSON is stored JSON, or JSON null for a null column.
func RawJSON(b []byte) json.RawMessage {
	if b == nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}
