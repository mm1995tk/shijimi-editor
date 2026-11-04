package config

// Section is the part of config.toml that is not the core's: a UI reads its
// own keys (theme, the terminal multiplexer, ...) from the same files. The
// core does not know what a section holds. It checks the keys with its own
// (a key that no one reads is a mistake), applies each file to the section's
// Value in the order it applies its own, and keeps the Value in the Config
// for the UI to read (Config.Value).
type Section struct {
	// Name identifies the section in Config.Value.
	Name string
	// Schema is the keys the section reads, by top-level key. A table that the
	// core has too ("editor") is merged with the core's: each key must have
	// one reader.
	Schema map[string]Field
	// New returns the section's Value with its defaults.
	New func() Value
}

// Value is what a section reads from a file.
type Value interface {
	// Merge applies a decoded config.toml / config.nix. user is whether it is
	// from the user's own directory. The schema has been checked already.
	Merge(raw map[string]any, user bool)
}

// Value returns what the section of that name read, or nil when the section
// was not registered (Options.Sections).
func (c *Config) Value(name string) Value {
	if c == nil {
		return nil
	}
	return c.values[name]
}

// SetValue puts v as the section's Value, for a Config made by hand (tests).
func (c *Config) SetValue(name string, v Value) {
	if c.values == nil {
		c.values = map[string]Value{}
	}
	c.values[name] = v
}

// withSections returns the schema of config.toml with the keys of the sections,
// and a new Value for each section.
func withSections(secs []Section) (map[string]field, map[string]Value) {
	schema := configSchema
	values := make(map[string]Value, len(secs))
	if len(secs) == 0 {
		return schema, values
	}
	schema = mergeSchema(nil, configSchema)
	for _, s := range secs {
		schema = mergeSchema(schema, s.Schema)
		values[s.Name] = s.New()
	}
	return schema, values
}

// mergeSchema adds src to dst (a copy is made when dst is nil), merging the
// tables both have.
func mergeSchema(dst, src map[string]field) map[string]field {
	if dst == nil {
		dst = make(map[string]field, len(src))
	}
	for k, f := range src {
		if old, ok := dst[k]; ok && old.kind == kTable && f.kind == kTable {
			f.sub = mergeSchema(mergeSchema(nil, old.sub), f.sub)
			f.userOnly = f.userOnly || old.userOnly
		}
		dst[k] = f
	}
	return dst
}

// Field is what a setting must be (see Section.Schema).
type Field = field

// The types of a setting.
var (
	String  = fString
	Bool    = fBool
	Int     = fInt
	Strings = fStrings
)

// Table is a table with the keys of sub.
func Table(sub map[string]Field) Field { return table(sub) }

// UserOnly marks the setting as read only from the user's own directory.
func (f Field) UserOnly() Field {
	f.userOnly = true
	return f
}

// Check sets the test of a value of the right type. fn returns the i18n key
// of the reason (and its arguments) when the value is not acceptable, "" when
// it is.
func (f Field) Check(fn func(v any) (code string, args []any)) Field {
	f.check = fn
	return f
}

// NonNegative is a Check that accepts an integer that is 0 or more.
func NonNegative(v any) (string, []any) { return checkNonNegative(v) }

// StringList is the strings of a decoded array, without the empty ones.
func StringList(v any) []string { return stringList(v) }

// Tables is an array of tables ([[name]]) with the keys of sub. Each table
// must have the keys in required.
func Tables(required []string, sub map[string]Field) Field {
	return field{kind: kTables, required: required, sub: sub}
}
