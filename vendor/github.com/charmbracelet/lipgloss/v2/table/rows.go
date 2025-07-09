package table

// Data is the interface that wraps the basic methods of a table model.
type Data interface {
	// At returns the contents of the cell at the given index.
	At(row, cell int) string

	// Rows returns the number of rows in the table.
	Rows() int

	// Columns returns the number of columns in the table.
	Columns() int
}

// StringData is a string-based implementation of the Data interface.
type StringData struct {
	rows    [][]string
	columns int
}

// NewStringData creates a new StringData with the given number of columns.
func NewStringData(rows ...[]string) *StringData {
	m := StringData{columns: 0}

	for _, row := range rows {
		m.columns = max(m.columns, len(row))
		m.rows = append(m.rows, row)
	}

	return &m
}

// Append appends the given row to the table.
func (m *StringData) Append(row []string) {
	m.columns = max(m.columns, len(row))
	m.rows = append(m.rows, row)
}

// At returns the contents of the cell at the given index.
func (m *StringData) At(row, cell int) string {
	if row >= len(m.rows) || cell >= len(m.rows[row]) {
		return ""
	}

	return m.rows[row][cell]
}

// Columns returns the number of columns in the table.
func (m *StringData) Columns() int {
	return m.columns
}

// Item appends the given row to the table.
func (m *StringData) Item(rows ...string) *StringData {
	m.columns = max(m.columns, len(rows))
	m.rows = append(m.rows, rows)
	return m
}

// Rows returns the number of rows in the table.
func (m *StringData) Rows() int {
	return len(m.rows)
}

// Filter applies a filter on some data.
type Filter struct {
	data   Data
	filter func(row int) bool
}

// NewFilter initializes a new Filter.
func NewFilter(data Data) *Filter {
	return &Filter{data: data}
}

// Filter applies the given filter function to the data.
func (m *Filter) Filter(f func(row int) bool) *Filter {
	m.filter = f
	return m
}

// At returns the row at the given index.
func (m *Filter) At(row, cell int) string {
	j := 0
	for i := range m.data.Rows() {
		if m.filter(i) {
			if j == row {
				return m.data.At(i, cell)
			}

			j++
		}
	}

	return ""
}

// Columns returns the number of columns in the table.
func (m *Filter) Columns() int {
	return m.data.Columns()
}

// Rows returns the number of rows in the table.
func (m *Filter) Rows() int {
	j := 0
	for i := range m.data.Rows() {
		if m.filter(i) {
			j++
		}
	}

	return j
}

// DataToMatrix is a helper function that converts an object that implements the
// Data interface into a table.
func DataToMatrix(data Data) (rows [][]string) {
	numRows := data.Rows()
	numCols := data.Columns()
	rows = make([][]string, numRows)

	for i := range numRows {
		rows[i] = make([]string, numCols)

		for j := range numCols {
			rows[i][j] = data.At(i, j)
		}
	}
	return
}
