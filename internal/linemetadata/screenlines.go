package linemetadata

// ScreenLines represents a vertical count of physical rows on the terminal
// screen.
//
// This is explicitly distinct from Index and Number, which represent logical
// lines from the input stream. A single input line (Index) can wrap into
// multiple ScreenLines if it contains more characters than the terminal is
// wide.
//
// Because a ScreenLines value represents a geometry constraint or visual
// offset, it should never be added directly to an Index or Number calculation.
//
// It can represent an absolute measure (like terminal height) or a relative
// offset (which can be negative, such as a scroll delta).
type ScreenLines int
