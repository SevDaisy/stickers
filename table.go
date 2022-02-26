package stickers

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"log"
	"reflect"
	"strconv"
	"strings"
)

var (
	tableDefaultHeaderStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7158e2")).
				Foreground(lipgloss.Color("#ffffff"))
	tableDefaultFooterStyle = tableDefaultHeaderStyle.Copy().Align(lipgloss.Right).Height(1)
	tableDefaultRowsStyle   = lipgloss.NewStyle().
				Background(lipgloss.Color("#4b4b4b")).
				Foreground(lipgloss.Color("#ffffff"))
	tableDefaultRowsSubsequentStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("#3d3d3d")).
					Foreground(lipgloss.Color("#ffffff"))
	tableDefaultRowsCursorStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("#f7b731")).
					Foreground(lipgloss.Color("#000000")).
					Bold(true)
	tableDefaultCellCursorStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("#f6e58d")).
					Foreground(lipgloss.Color("#000000"))
)

type tableStyleKey int

const (
	TableHeaderStyleKey tableStyleKey = iota
	TableFooterStyleKey
	TableRowsStyleKey
	TableRowsSubsequentStyleKey
	TableRowsCursorStyleKey
	TableCellCursorStyleKey
)

type TableSortingOrderKey int

const (
	TableSortingAscending  = 0
	TableSortingDescending = 1
)

type Ordered interface {
	int | int8 | int32 | int16 | int64 | float32 | float64 | string
}

// TableBadTypeError type does not match Ordered interface types
type TableBadTypeError struct {
	msg string
}

func (e TableBadTypeError) Error() string {
	return e.msg
}

// TableRowLenError row length is not matching headers len
type TableRowLenError struct {
	msg string
}

func (e TableRowLenError) Error() string {
	return e.msg
}

// TableBadCellTypeError type of cell does not match type of column
type TableBadCellTypeError struct {
	msg string
}

func (e TableBadCellTypeError) Error() string {
	return e.msg
}

// Table responsive, x/y scrollable table that uses magic of FlexBox
type Table struct {
	// columnRatio ratio of the columns, is applied to rows as well
	columnRatio []int
	// columnMinWidth minimal width of the column
	columnMinWidth []int
	// columnHeaders column text headers
	// TODO: make this optional, as well as footer
	columnHeaders []string
	columnType    []any
	rows          [][]any

	// orderColumnIndex notes which column is used for sorting
	// -1 means that no column is sorted
	orderedColumnIndex int
	// orderedColumnPhase remarks if the sort is asc or desc, basically works like a toggle
	// 0 indicates desc sorting, 1 indicates
	orderedColumnPhase TableSortingOrderKey

	// rowsTopIndex top visible index
	rowsTopIndex int
	cursorIndexY int
	cursorIndexX int

	height int
	width  int

	rowsBoxHeight int
	// rowHeight fixed row height value, maybe this should be optional?
	rowHeight int

	styles map[tableStyleKey]lipgloss.Style

	headerBox *FlexBox
	rowsBox   *FlexBox

	// these flags indicate weather we should update rows and headers flex boxes
	updateRowsFlag    bool
	updateHeadersFlag bool
}

// NewTable initialize Table object with defaults
func NewTable(width, height int, columnHeaders []string) *Table {
	var columnRatio, columnMinWidth []int
	for _ = range columnHeaders {
		columnRatio = append(columnRatio, 1)
		columnMinWidth = append(columnMinWidth, 0)
	}

	// by default all columns are of type string
	var defaultType string
	var defaultTypes []any
	for range columnHeaders {
		defaultTypes = append(defaultTypes, defaultType)
	}

	styles := map[tableStyleKey]lipgloss.Style{
		TableHeaderStyleKey:         tableDefaultHeaderStyle,
		TableFooterStyleKey:         tableDefaultFooterStyle,
		TableRowsStyleKey:           tableDefaultRowsStyle,
		TableRowsSubsequentStyleKey: tableDefaultRowsSubsequentStyle,
		TableRowsCursorStyleKey:     tableDefaultRowsCursorStyle,
		TableCellCursorStyleKey:     tableDefaultCellCursorStyle,
	}

	r := &Table{
		columnHeaders:  columnHeaders,
		columnRatio:    columnRatio,
		columnMinWidth: columnMinWidth,
		cursorIndexX:   0,
		cursorIndexY:   0,

		columnType:         defaultTypes,
		orderedColumnIndex: -1,
		orderedColumnPhase: TableSortingDescending,

		height: height,
		width:  width,
		// when optional header/footer is set rework this
		rowsBoxHeight: height - 2,

		rowsTopIndex: 0,
		rowHeight:    1,

		headerBox: NewFlexBox(width, 1).SetStyle(tableDefaultHeaderStyle),
		rowsBox:   NewFlexBox(width, height-1),

		styles: styles,
	}
	r.setHeadersUpdate()
	return r
}

// SetRatio replaces the ratio slice, it has to be exactly the len of the headers/rows slices
// if it's not matching len it will trigger fatal error
func (r *Table) SetRatio(values []int) *Table {
	if len(values) != len(r.columnHeaders) {
		log.Fatalf("ratio list[%d] not of proper length[%d]\n", len(values), len(r.columnHeaders))
	}
	r.columnRatio = values
	r.setHeadersUpdate()
	r.setRowsUpdate()
	return r
}

// SetTypes sets the column type, setting this will remove all the rows so make sure you do it when instantiating
// Table object or add new rows after this, types have to be one of Ordered interface types
func (r *Table) SetTypes(columnTypes ...any) (*Table, error) {
	if len(columnTypes) != len(r.columnHeaders) {
		return r, errors.New("column types not the same len as headers")
	}
	for i, t := range columnTypes {
		if !isOrdered(t) {
			message := fmt.Sprintf(
				"column of type %s on index %d is not of type Ordered", reflect.TypeOf(t).String(), i,
			)
			return r, TableBadTypeError{msg: message}
		}
	}
	r.cursorIndexY, r.cursorIndexX = 0, 0
	r.rows = [][]any{}
	r.columnType = columnTypes
	r.setRowsUpdate()
	return r, nil
}

// SetMinWidth replaces the minimum width slice, it has to be exactly the len of the headers/rows slices
// if it's not matching len it will trigger fatal error
func (r *Table) SetMinWidth(values []int) *Table {
	if len(values) != len(r.columnHeaders) {
		log.Fatalf("min width list[%d] not of proper length[%d]\n", len(values), len(r.columnHeaders))
	}
	r.columnMinWidth = values
	r.setHeadersUpdate()
	r.setRowsUpdate()
	return r
}

// SetHeight sets the height of the table including the header and footer
func (r *Table) SetHeight(value int) *Table {
	r.height = value
	// we deduct two to take header/footer into the account
	r.rowsBoxHeight = value - 2
	r.rowsBox.SetHeight(r.rowsBoxHeight)
	r.setRowsUpdate()
	return r
}

// SetWidth sets the width of the table
func (r *Table) SetWidth(value int) *Table {
	r.width = value
	r.rowsBox.SetWidth(value)
	r.headerBox.SetWidth(value)
	return r
}

// CursorDown move table cursor down
func (r *Table) CursorDown() *Table {
	if r.cursorIndexY+1 < len(r.rows) {
		r.cursorIndexY++
		r.setTopRow()
		r.setRowsUpdate()
	}
	return r
}

// CursorUp move table cursor up
func (r *Table) CursorUp() *Table {
	if r.cursorIndexY-1 > -1 {
		r.cursorIndexY--
		r.setTopRow()
		r.setRowsUpdate()
	}
	return r
}

// CursorLeft move table cursor left
func (r *Table) CursorLeft() *Table {
	if r.cursorIndexX-1 > -1 {
		r.cursorIndexX--
		// TODO: update row only
		r.setRowsUpdate()
	}
	return r
}

// CursorRight move table cursor right
func (r *Table) CursorRight() *Table {
	if r.cursorIndexX+1 < len(r.columnHeaders) {
		r.cursorIndexX++
		// TODO: update row only
		r.setRowsUpdate()
	}
	return r
}

// GetCursorLocation returns the current x,y position of the cursor
func (r *Table) GetCursorLocation() (int, int) {
	return r.cursorIndexX, r.cursorIndexY
}

// GetCursorValue returns the string of the cell under the cursor
func (r *Table) GetCursorValue() string {
	// handle 0 rows situation and when table is not active
	if len(r.rows) == 0 || r.cursorIndexX < 0 || r.cursorIndexY < 0 {
		return ""
	}
	return getStringFromOrdered(r.rows[r.cursorIndexY][r.cursorIndexX])
}

// AddRows add multiple rows, will return error on the first instance of a row that does not match the type set on table
// will update rows only when there are no errors
func (r *Table) AddRows(rows [][]any) (*Table, error) {
	// check for errors
	for _, row := range rows {
		if err := r.validateRow(row...); err != nil {
			return r, err
		}
	}
	// append rows
	for _, row := range rows {
		r.rows = append(r.rows, row)
	}

	r.setRowsUpdate()
	return r, nil
}

// MustAddRows executes AddRows and panics if there is an error
func (r *Table) MustAddRows(rows [][]any) *Table {
	if _, err := r.AddRows(rows); err != nil {
		panic(err)
	}
	return r
}

// OrderByColumn orders rows by a column with the index n, simple bubble sort, nothing too fancy
// does not apply when there is less than 2 row in a table
// TODO: this messes up numbering that one might use, implement automatic indexing of rows
// TODO: allow user to disable ordering
func (r *Table) OrderByColumn(index int) *Table {
	// sanity check first, we won't return errors here, simply ignore if the user sends non existing index
	if index < len(r.columnHeaders) && len(r.rows) > 1 {
		r.updateOrderedVars(index)

		// sorted rows
		var sorted [][]any
		// list of column values used for ordering
		var orderingCol []any
		for _, rw := range r.rows {
			orderingCol = append(orderingCol, rw[index])
		}
		// get sorting index
		sortingIndex := sortIndexByOrderedColumn(orderingCol, r.orderedColumnPhase)
		// update rows
		for _, i := range sortingIndex {
			sorted = append(sorted, r.rows[i])
		}
		r.rows = sorted
		r.setRowsUpdate()
	}
	return r
}

// Render renders the table into the string
func (r *Table) Render() string {
	r.updateRows()
	r.updateHeader()
	return lipgloss.JoinVertical(
		lipgloss.Left,
		r.headerBox.Render(),
		r.rowsBox.Render(),
		r.styles[TableFooterStyleKey].
			Width(r.width).
			Render(
				fmt.Sprintf(
					"%d:%d / %d:%d ",
					r.cursorIndexX,
					r.cursorIndexY,
					r.rowsBox.GetWidth(),
					r.rowsBox.GetHeight(),
				),
			),
	)
}

func (r *Table) setRowsUpdate() {
	r.updateRowsFlag = true
}

func (r *Table) unsetRowsUpdate() {
	r.updateRowsFlag = false
}

func (r *Table) setHeadersUpdate() {
	r.updateHeadersFlag = true
}

func (r *Table) unsetHeadersUpdate() {
	r.updateHeadersFlag = false
}

// validateRow checks the row for validity, number of cells must match table header length
// and header types per cell as well
func (r *Table) validateRow(cells ...any) error {
	var message string
	// check row len
	if len(cells) != len(r.columnType) {
		message = fmt.Sprintf(
			"len of row[%d] does not equal number of columns[%d]", len(cells), len(r.columnType),
		)
		return TableRowLenError{msg: message}
	}
	// check cell type
	for i, c := range cells {
		switch c.(type) {
		case string, int, int8, int16, int32, float32, float64:
			// check if the cell matches the type of the column
			if reflect.TypeOf(c) != reflect.TypeOf(r.columnType[i]) {
				message = fmt.Sprintf(
					"type of the cell[%v] on index %d not matching type of the column[%v]",
					reflect.TypeOf(c), i, reflect.TypeOf(r.columnType[i]),
				)
				return TableBadCellTypeError{msg: message}
			}
		default:
			message = fmt.Sprintf(
				"type[%v] on index %d not matching Ordered interface types", reflect.TypeOf(c), i,
			)
			return TableBadTypeError{msg: message}
		}
	}
	return nil
}

// updateHeader recomputes the header of the table
func (r *Table) updateHeader() *Table {
	if !r.updateHeadersFlag {
		return r
	}
	var cells []*FlexBoxCell
	r.headerBox.SetStyle(r.styles[TableHeaderStyleKey])
	for i, title := range r.columnHeaders {
		cells = append(
			cells,
			NewFlexBoxCell(r.columnRatio[i], 1).SetMinWidth(r.columnMinWidth[i]).SetContent(title),
		)
	}
	r.headerBox.SetRows(
		[]*FlexBoxRow{
			r.headerBox.NewRow().AddCells(cells),
		},
	)
	r.unsetHeadersUpdate()
	return r
}

// updateRows recomputes the rows of the table
// calculate the visible rows top/bottom indexes
// create rows and their cells with styles depending on state
func (r *Table) updateRows() {
	if !r.updateRowsFlag {
		return
	}
	if r.rowsBoxHeight < 0 {
		r.unsetRowsUpdate()
		return
	}

	// calculate the bottom most visible row index
	rowsBottomIndex := r.rowsTopIndex + r.rowsBoxHeight
	if rowsBottomIndex > len(r.rows) {
		rowsBottomIndex = len(r.rows)
	}

	var rows []*FlexBoxRow
	for ir, columns := range r.rows[r.rowsTopIndex:rowsBottomIndex] {
		// irCorrected is corrected row index since we iterate only visible rows
		irCorrected := ir + r.rowsTopIndex

		var cells []*FlexBoxCell
		for ic, column := range columns {
			// initialize column cell
			c := NewFlexBoxCell(r.columnRatio[ic], r.rowHeight).
				SetMinWidth(r.columnMinWidth[ic]).
				SetContent(getStringFromOrdered(column))
			// update style if cursor is on the cell, otherwise it's inherited from the row
			if irCorrected == r.cursorIndexY && ic == r.cursorIndexX {
				c.SetStyle(r.styles[TableCellCursorStyleKey])
			}
			cells = append(cells, c)
		}
		// initialize new row from the rows box and add generated cells
		rw := r.rowsBox.NewRow().AddCells(cells)

		// rows have three styles, normal, subsequent and selected
		// normal and subsequent rows should differ for readability
		// TODO: make this ^ optional
		if irCorrected == r.cursorIndexY {
			rw.SetStyle(r.styles[TableRowsCursorStyleKey])
		} else if irCorrected%2 == 0 || irCorrected == 0 {
			rw.SetStyle(r.styles[TableRowsSubsequentStyleKey])
		} else {
			rw.SetStyle(r.styles[TableRowsStyleKey])
		}

		rows = append(rows, rw)
	}

	// lock row height, this might get optional at some point
	r.rowsBox.LockRowHeight(r.rowHeight)
	r.rowsBox.SetRows(rows)
	r.unsetRowsUpdate()
	return
}

// updateOrderedVars updates bits and pieces revolving around ordering
// toggling between asc and desc
// updating column header with arrows
// updating ordering vars on TableOrdered
func (r *Table) updateOrderedVars(index int) {
	// strip the column header title arrows if they were set previously
	if r.orderedColumnIndex > -1 {
		// always expect two characters, nothing fancy
		r.columnHeaders[r.orderedColumnIndex] = strings.TrimSuffix(r.columnHeaders[r.orderedColumnIndex], " ▲")
		r.columnHeaders[r.orderedColumnIndex] = strings.TrimSuffix(r.columnHeaders[r.orderedColumnIndex], " ▼")
	}

	// toggle between ascending and descending and set default first sort to ascending
	// set updated column header title
	if r.orderedColumnIndex == index {
		switch r.orderedColumnPhase {
		case TableSortingAscending:
			r.orderedColumnPhase = TableSortingDescending
			r.columnHeaders[index] = r.columnHeaders[index] + " ▼"
		case TableSortingDescending:
			r.orderedColumnPhase = TableSortingAscending
			r.columnHeaders[index] = r.columnHeaders[index] + " ▲"
		}
	} else {
		r.orderedColumnPhase = TableSortingDescending
		r.columnHeaders[index] = r.columnHeaders[index] + " ▼"
	}
	r.orderedColumnIndex = index

	r.setHeadersUpdate()
}

// setTopRow calculates the row top index used when deciding what is visible
func (r *Table) setTopRow() {
	// if rows are empty set x and y to 0
	// will be useful for filtering
	if len(r.rows) == 0 {
		r.cursorIndexY = 0
		r.cursorIndexX = 0
	} else if r.cursorIndexY > len(r.rows) {
		// when filtering if cursor is higher than row length
		// set it to the bottom of the list
		r.cursorIndexY = len(r.rows) - 1
	}

	// case when cursor is in between top or bottom visible row
	if r.cursorIndexY >= r.rowsTopIndex && r.cursorIndexY < r.rowsTopIndex+r.rowsBoxHeight {
		// if cursor is on the last item in row, adjust the row top
		if r.cursorIndexY == len(r.rows)-1 {
			// if all rows can fit on screen
			if len(r.rows) <= r.rowsBoxHeight {
				r.rowsTopIndex = 0
				return
			}
			// fit max rows on the table
			r.rowsTopIndex = r.cursorIndexY - (r.rowsBoxHeight - 1)
		}
		return
	}

	// if cursor is above the top
	if r.cursorIndexY < r.rowsTopIndex {
		r.rowsTopIndex = r.cursorIndexY
		return
	}

	if r.cursorIndexY > r.rowsTopIndex {
		//log.Fatal(fmt.Sprintf("[%d][%d][%d]", r.rowsTopIndex, r.cursorIndexY, r.rowsBoxHeight))
		r.rowsTopIndex = r.cursorIndexY - r.rowsBoxHeight + 1
		return
	}
}

// isOrdered check if type is one of valid Ordered types
func isOrdered(e any) bool {
	switch e.(type) {
	case string, int, int8, int16, int32, float32, float64:
		return true
	default:
		return false
	}
}

// getStringFromOrdered returns string from interface that was produced with one of Ordered types
func getStringFromOrdered(i any) string {
	switch i.(type) {
	case string:
		return i.(string)
	case int:
		return strconv.Itoa(i.(int))
	case int8:
		return strconv.Itoa(int(i.(int8)))
	case int16:
		return strconv.Itoa(int(i.(int16)))
	case int32:
		return strconv.Itoa(int(i.(int32)))
	case int64:
		return strconv.Itoa(int(i.(int64)))
	case float32:
		// default precision of 24
		return strconv.FormatFloat(float64(i.(float32)), 'G', 0, 32)
	case float64:
		// default precision of 24
		return strconv.FormatFloat(i.(float64), 'G', 0, 64)
	default:
		return ""
	}
}

// sortIndexByOrderedColumn casts to the one of Ordered type that is used on the column and sends to sorting
// returns sorted index of elements rather than elements themselves
func sortIndexByOrderedColumn(i []any, order TableSortingOrderKey) (sortedIndex []int) {
	// if len of slice is 0 return empty sort order
	if len(i) == 0 {
		return sortedIndex
	}

	switch i[0].(type) {
	case string:
		var s []string
		for _, el := range i {
			s = append(s, el.(string))
		}
		return sortIndex(s, order)
	case int:
		var s []int
		for _, el := range i {
			s = append(s, el.(int))
		}
		return sortIndex(s, order)
	case int8:
		var s []int8
		for _, el := range i {
			s = append(s, el.(int8))
		}
		return sortIndex(s, order)
	case int16:
		var s []int16
		for _, el := range i {
			s = append(s, el.(int16))
		}
		return sortIndex(s, order)
	case int32:
		var s []int32
		for _, el := range i {
			s = append(s, el.(int32))
		}
		return sortIndex(s, order)
	case int64:
		var s []int64
		for _, el := range i {
			s = append(s, el.(int64))
		}
		return sortIndex(s, order)
	case float32:
		var s []float32
		for _, el := range i {
			s = append(s, el.(float32))
		}
		return sortIndex(s, order)
	case float64:
		var s []float64
		for _, el := range i {
			s = append(s, el.(float64))
		}
		return sortIndex(s, order)

	default:
		panic(fmt.Sprintf("type %s not subtype of Ordered", reflect.TypeOf(i[0]).String()))
	}
}

// sortIndex is simple generic bubble sort, returns sorted index slice
// bubble sort implemented for simplicity, if you need faster alg feel free to open a PR for it :zap:
func sortIndex[T Ordered](slice []T, order TableSortingOrderKey) []int {
	// could do this in sortIndexByOrderedColumn where we cycle through the slice anyhow
	// tho I think this is cheap op and makes code a bit cleaner, worthy trade for now
	var index []int
	for i := 0; i < len(slice); i++ {
		index = append(index, i)
	}

	// bubble sort slice and update index in a process
	for i := len(slice); i > 0; i-- {
		for j := 1; j < i; j++ {
			if order == TableSortingDescending && slice[j] < slice[j-1] {
				slice[j], slice[j-1] = slice[j-1], slice[j]
				index[j], index[j-1] = index[j-1], index[j]
			} else if order == TableSortingAscending && slice[j] > slice[j-1] {
				slice[j], slice[j-1] = slice[j-1], slice[j]
				index[j], index[j-1] = index[j-1], index[j]
			}
		}
	}
	return index
}
