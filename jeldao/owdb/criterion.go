package owdb

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"
)

// Criterion is match criteria for a single property of a Hit. Its Meets
// function performs the actual check as to whether the given value meets it.
//
// The Format string is used for printing the Criterion to a human-readable
// string. It will be passed the formatted string that gives the name of the
// property that will be checked against the criterion at the time of
// formatting; this will be "VALUE" for cases when there is no specific property
// being checked (such as when calling String() by itself). If Format is not
// set, a generic string will be used instead.
//
// NotFormat, if given, defines what to show when this Criterion has a Not
// applied to it. It is optional and Not will default to a generic string if not
// given.
//
// Both Format and NotFormat have the potential to be used for equality
// checking. Two Crtierion with the same Format strings should return the same
// values from their Meets methods when given identical inputs. The same applies
// to NotFormat.
//
// EstLimits gives the minimum and maximum values, according to some known
// ordering, that would be captured by this Criterion. It is used for query
// planning, and all Criterion that do not define it will not be able to be used
// for query planning or limiting, even if they are applied to an indexed field.
// This does not mean the Criterion will not be applied, just that the search
// for the initial set to apply it to will not use it for that purpose. Limits
// are always an estimate, even when defined - the real "minimum" is the
// smallest one that meets the criterion, which may not always be easily
// determinable. Limits must always be no narrower than the set of input the
// Criterion matches on, but they may be wider.
type Criterion[E any] struct {
	Meets     func(v E) bool
	Format    string
	NotFormat string
	EstLimits Limits[E]
}

// String returns the string representation of crit, which will be the same as
// CheckString called with a placeholder string.
func (crit Criterion[E]) String() string {
	return crit.FilledString("VALUE")
}

// FilledString returns the string representation of this Criterion when it is
// being used to check against a particular value. The value could be the name
// of a property, or the string representation of an actual value along with any
// delimiter characters that show it. This value is passed unchanged to crit's
// Format to create the formatted check-string. If Format is set to the empty
// string, a generic format string is used instead.
func (crit Criterion[E]) FilledString(value string) string {
	fmtStr := crit.Format
	if fmtStr == "" {
		fmtStr = "CRITERION(%s)"
	}

	return fmt.Sprintf(fmtStr, value)
}

// EqualsIP returns a Criterion that checks that the net.IP property of interest
// equals the address in the given value. For the purposes of this function, an
// IPv4 address and that same address in IPv6 form are considered the same
// address. Panics if addr is not a parsable IPv4 or IPv6 address, if you want
// to check for a nil IP on a hit, use [IsNullIP] instead.
func EqualsIP(addr string) Criterion[net.IP] {
	ip := net.ParseIP(addr)
	if ip == nil {
		panic(fmt.Sprintf("not a valid IP address: %q", addr))
	}

	return Criterion[net.IP]{
		Meets: func(v net.IP) bool {
			if v == nil {
				return false
			}
			return ip.Equal(v)
		},
		Format:    "%s" + fmt.Sprintf(" == %s", ip),
		NotFormat: "%s" + fmt.Sprintf(" != %s", ip),
		EstLimits: Limits[net.IP]{Min: &ip, Max: &ip},
	}
}

// IsGreaterThanIP returns a Criterion that checks that the net.IP property of
// interest comes after the given value when the two are compared on a
// byte-by-byte level. Both are converted to full IPv6 representations before
// the comparison is made. A nil IP address is considered to be less than all
// other addresses. Panics if addr is not a parsable IPv4 or IPv6 address.
func IsGreaterThanIP(addr string) Criterion[net.IP] {
	ip := net.ParseIP(addr)
	if ip == nil {
		panic(fmt.Sprintf("not a valid IP address: %q", addr))
	}

	return Criterion[net.IP]{
		Meets: func(v net.IP) bool {
			if v == nil {
				return false
			}
			ip6 := ip.To16()
			v6 := v.To16()

			for i := 0; i < 16; i++ {
				if v6[i] > ip6[i] {
					return true
				}
				if v6[i] < ip6[i] {
					return false
				}
			}

			// got through all? then they are equal, which is not >.
			return false
		},
		Format:    "%s" + fmt.Sprintf(" > %s", ip),
		NotFormat: "%s" + fmt.Sprintf(" <= %s", ip),
		EstLimits: Limits[net.IP]{Min: &ip},
	}
}

// IsGreaterThanOrEqualsIP returns a Criterion that checks that the net.IP
// property of interest comes after the given value when the two are compared on
// a byte-by-byte level, or that the two are equal. Both are converted to full
// IPv6 representations before the comparison is made. A nil IP address is
// considered to be less than all other addresses. Panics if addr is not a
// parsable IPv4 or IPv6 address.
func IsGreaterThanOrEqualsIP(addr string) Criterion[net.IP] {
	ip := net.ParseIP(addr)
	if ip == nil {
		panic(fmt.Sprintf("not a valid IP address: %q", addr))
	}

	return Criterion[net.IP]{
		Meets: func(v net.IP) bool {
			if v == nil {
				return false
			}

			if ip.Equal(v) {
				return true
			}

			ip6 := ip.To16()
			v6 := v.To16()

			for i := 0; i < 16; i++ {
				if v6[i] > ip6[i] {
					return true
				}
				if v6[i] < ip6[i] {
					return false
				}
			}

			// got through all? then they are equal
			return true
		},
		Format:    "%s" + fmt.Sprintf(" >= %s", ip),
		NotFormat: "%s" + fmt.Sprintf(" < %s", ip),
		EstLimits: Limits[net.IP]{Min: &ip},
	}
}

// IsLessThanIP returns a Criterion that checks that the net.IP property of
// interest comes before the given value when the two are compared on a
// byte-by-byte level. Both are converted to full IPv6 representations before
// the comparison is made. A nil IP address is considered to be less than all
// other addresses. Panics if addr is not a parsable IPv4 or IPv6 address.
func IsLessThanIP(addr string) Criterion[net.IP] {
	ip := net.ParseIP(addr)
	if ip == nil {
		panic(fmt.Sprintf("not a valid IP address: %q", addr))
	}

	return Criterion[net.IP]{
		Meets: func(v net.IP) bool {
			if v == nil {
				return true
			}

			ip6 := ip.To16()
			v6 := v.To16()

			for i := 0; i < 16; i++ {
				if v6[i] < ip6[i] {
					return true
				}
				if v6[i] > ip6[i] {
					return false
				}
			}

			// got through all? then they are equal, which is not <.
			return false
		},
		Format:    "%s" + fmt.Sprintf(" < %s", ip),
		NotFormat: "%s" + fmt.Sprintf(" >= %s", ip),
		EstLimits: Limits[net.IP]{Max: &ip},
	}
}

// IsLessThanOrEqualsIP returns a Criterion that checks that the net.IP property
// of interest comes before the given value when the two are compared on a
// byte-by-byte level, or that the two are equal. Both are converted to full
// IPv6 representations before the comparison is made. A nil IP address is
// considered to be less than all other addresses. Panics if addr is not a
// parsable IPv4 or IPv6 address.
func IsLessThanOrEqualsIP(addr string) Criterion[net.IP] {
	ip := net.ParseIP(addr)
	if ip == nil {
		panic(fmt.Sprintf("not a valid IP address: %q", addr))
	}

	return Criterion[net.IP]{
		Meets: func(v net.IP) bool {
			if v == nil {
				return true
			}

			if ip.Equal(v) {
				return true
			}

			ip6 := ip.To16()
			v6 := v.To16()

			for i := 0; i < 16; i++ {
				if v6[i] < ip6[i] {
					return true
				}
				if v6[i] > ip6[i] {
					return false
				}
			}

			// got through all? then they are equal
			return true
		},
		Format:    "%s" + fmt.Sprintf(" <= %s", ip),
		NotFormat: "%s" + fmt.Sprintf(" > %s", ip),
		EstLimits: Limits[net.IP]{Max: &ip},
	}
}

// IsBetweenIPs returns a Criterion that checks that the net.IP property of
// interest lies between the two addresses, inclusive. Equality checks are
// performed as per EqualsIP, and less than and greater than checks are
// performed as per IsLessThanIP and IsGreaterThanIP. Panics if either start or
// end is not a parsable IPv4 or IPv6 address.
func IsBetweenIPs(start, end string) Criterion[net.IP] {
	startIP := net.ParseIP(start)
	if startIP == nil {
		panic(fmt.Sprintf("not a valid IP address: %q", start))
	}
	endIP := net.ParseIP(end)
	if endIP == nil {
		panic(fmt.Sprintf("not a valid IP address: %q", end))
	}

	return Criterion[net.IP]{
		Meets: func(v net.IP) bool {
			if v == nil {
				return false
			}

			end6 := endIP.To16()
			v6 := v.To16()

			// first, is it >= start?
			start6 := startIP.To16()

			if !start6.Equal(v6) {
				// if it's not equal, it betta be gr8er!!!!!!!!
				for i := 0; i < 16; i++ {
					if v6[i] < start6[i] {
						return false
					}
					if v6[i] > start6[i] {
						break
						// it's greater than, no need to keep checking
					}
				}
			}

			// next, is it <= end?
			if !end6.Equal(v6) {
				// if it's not equal, it betta be less!!!!!!!!
				for i := 0; i < 16; i++ {
					if v6[i] > end6[i] {
						return false
					}
					if v6[i] < end6[i] {
						break
						// it's less than, no need to keep checking
					}
				}
			}

			// no check failed. it lies between them
			return true
		},
		Format:    fmt.Sprintf("%s <= ", startIP) + "%s" + fmt.Sprintf(" <= %s", endIP),
		NotFormat: fmt.Sprintf("%s <= ", startIP) + "%s" + fmt.Sprintf(" <= %s", endIP),
		EstLimits: Limits[net.IP]{Min: &startIP, Max: &endIP},
	}
}

// IsNullIP returns a Criterion that checks that the net.IP property of interest
// is not set.
func IsNullIP() Criterion[net.IP] {
	return Criterion[net.IP]{
		Meets: func(v net.IP) bool {
			return v == nil
		},
		Format:    "%s == NULL",
		NotFormat: "%s != NULL",
		// no estimated limits defined; NULL check
	}
}

// EqualsString returns a Criterion that checks that the string property of
// interest exactly equals the given value.
func EqualsString(s string) Criterion[string] {
	return Criterion[string]{
		Meets: func(v string) bool {
			return v == s
		},
		Format:    "%s" + fmt.Sprintf(" == %q", s),
		NotFormat: "%s" + fmt.Sprintf(" != %q", s),
		EstLimits: Limits[string]{Min: &s, Max: &s},
	}
}

// CollatesAfter returns a Criterion that checks that the string property of
// interest comes after the given value.
func CollatesAfter(s string) Criterion[string] {
	return Criterion[string]{
		Meets: func(v string) bool {
			return v > s
		},
		Format:    "%s" + fmt.Sprintf(" > %q", s),
		NotFormat: "%s" + fmt.Sprintf(" <= %q", s),
		EstLimits: Limits[string]{Min: &s},
	}
}

// CollatesAfterOrEquals returns a Criterion that checks that the string
// property of interest comes after the given value or is the given value.
func CollatesAfterOrEquals(s string) Criterion[string] {
	return Criterion[string]{
		Meets: func(v string) bool {
			return v >= s
		},
		Format:    "%s" + fmt.Sprintf(" >= %q", s),
		NotFormat: "%s" + fmt.Sprintf(" < %q", s),
		EstLimits: Limits[string]{Min: &s},
	}
}

// CollatesBefore returns a Criterion that checks that the string property of
// interest comes before the given value.
func CollatesBefore(s string) Criterion[string] {
	return Criterion[string]{
		Meets: func(v string) bool {
			return v < s
		},
		Format:    "%s" + fmt.Sprintf(" < %q", s),
		NotFormat: "%s" + fmt.Sprintf(" >= %q", s),
		EstLimits: Limits[string]{Max: &s},
	}
}

// CollatesBefore returns a Criterion that checks that the string property of
// interest comes before or equals the given value.
func CollatesBeforeOrEquals(s string) Criterion[string] {
	return Criterion[string]{
		Meets: func(v string) bool {
			return v <= s
		},
		Format:    "%s" + fmt.Sprintf(" <= %q", s),
		NotFormat: "%s" + fmt.Sprintf(" > %q", s),
		EstLimits: Limits[string]{Max: &s},
	}
}

// CollatesBetween returns a Criterion that the time-based property of interest
// be between the given values, inclusive.
func CollatesBetween(start, end string) Criterion[string] {
	return Criterion[string]{
		Meets: func(v string) bool {
			return start <= v && v <= end
		},
		Format:    fmt.Sprintf("%q <= ", start) + "%s" + fmt.Sprintf(" >= %q", end),
		NotFormat: "!(" + fmt.Sprintf("%q <= ", start) + "%s" + fmt.Sprintf(" >= %q", end) + ")",
		EstLimits: Limits[string]{Min: &start, Max: &end},
	}
}

// IsNullString returns a Criterion that checks that the string property of
// interest is not set. This is equivalent to EqualsString(""), but does not
// have any estimated limits defined.
func IsNullString() Criterion[string] {
	return Criterion[string]{
		Meets: func(v string) bool {
			return v == ""
		},
		Format:    "%s == NULL",
		NotFormat: "%s != NULL",
		// no estimated limits defined; NULL check
	}
}

// EqualsTime returns a Criterion that the time-based property of interest be
// exactly the given value.
func EqualsTime(val time.Time) Criterion[time.Time] {
	val = val.UTC().Round(0)

	return Criterion[time.Time]{
		Meets: func(v time.Time) bool {
			return v == val
		},
		Format:    "%s" + fmt.Sprintf(" == %s", val.Format(time.RFC3339)),
		NotFormat: "%s" + fmt.Sprintf(" != %s", val.Format(time.RFC3339)),
		EstLimits: Limits[time.Time]{Min: &val, Max: &val},
	}
}

// IsAfter returns a Criterion that the time-based property of interest be
// after the given time, non-inclusive.
func IsAfter(t time.Time) Criterion[time.Time] {
	t = t.UTC().Round(0)

	return Criterion[time.Time]{
		Meets: func(v time.Time) bool {
			return v.After(t)
		},
		Format:    "%s > " + t.Format(time.RFC3339),
		NotFormat: "%s <= " + t.Format(time.RFC3339),
		EstLimits: Limits[time.Time]{Min: &t},
	}
}

// IsAfterOrEquals returns a Criterion that the time-based property of interest be
// on or after the given time.
func IsAfterOrEquals(t time.Time) Criterion[time.Time] {
	t = t.UTC().Round(0)

	return Criterion[time.Time]{
		Meets: func(v time.Time) bool {
			return v.After(t) || v == t
		},
		Format:    "%s >= " + t.Format(time.RFC3339),
		NotFormat: "%s < " + t.Format(time.RFC3339),
		EstLimits: Limits[time.Time]{Min: &t},
	}
}

// IsBefore returns a Criterion that the time-based property of interest be
// before the given time, non-inclusive.
func IsBefore(t time.Time) Criterion[time.Time] {
	t = t.UTC().Round(0)

	return Criterion[time.Time]{
		Meets: func(v time.Time) bool {
			return v.Before(t)
		},
		Format:    "%s < " + t.Format(time.RFC3339),
		NotFormat: "%s >= " + t.Format(time.RFC3339),
		EstLimits: Limits[time.Time]{Min: &t},
	}
}

// IsBeforeOrEquals returns a Criterion that the time-based property of interest be
// on or before the given time.
func IsBeforeOrEquals(t time.Time) Criterion[time.Time] {
	t = t.UTC().Round(0)

	return Criterion[time.Time]{
		Meets: func(v time.Time) bool {
			return v.Before(t) || v == t
		},
		Format:    "%s <= " + t.Format(time.RFC3339),
		NotFormat: "%s > " + t.Format(time.RFC3339),
		EstLimits: Limits[time.Time]{Max: &t},
	}
}

// IsBetweenTimes returns a Criterion that the time-based property of interest
// be between the given times, inclusive.
func IsBetweenTimes(start, end time.Time) Criterion[time.Time] {
	start = start.UTC().Round(0)
	end = end.UTC().Round(0)

	return Criterion[time.Time]{
		Meets: func(v time.Time) bool {
			return (v == start || v.After(start)) && (v == end || v.Before(end))
		},
		Format:    start.Format(time.RFC3339) + " <= %s >= " + end.Format(time.RFC3339),
		NotFormat: "!(" + start.Format(time.RFC3339) + " <= %s >= " + end.Format(time.RFC3339) + ")",
		EstLimits: Limits[time.Time]{Min: &start, Max: &end},
	}
}

// IsNullTime returns a Criterion that checks that the time property of interest
// is not set. This is equivalent to EqualsTime(time.Time{}), but does not have
// any estimated limits defined.
func IsNullTime() Criterion[time.Time] {
	return Criterion[time.Time]{
		Meets:     time.Time.IsZero,
		Format:    "%s == NULL",
		NotFormat: "%s != NULL",
		// no estimated limits defined; NULL check
	}
}

func DoesNot[E any](c Criterion[E]) Criterion[E] {
	origFormat := c.Format
	if origFormat == "" {
		// use a generic string instead, and inject a specifier
		origFormat = "CRITERION(%s)"
	}

	origNot := c.NotFormat
	if origNot == "" {
		origNot = "!(" + origFormat + ")" // do not run sprintf, origFormat is expected to contain specifiers
	}

	return Criterion[E]{
		Meets: func(v E) bool {
			return !c.Meets(v)
		},
		Format:    origNot,
		NotFormat: origFormat,
	}
}

// Meets returns a Criterion that matches against input by using the provided
// function as its Meets field. This is a convenience function for defining a
// new Criterion on-the-fly when the caller does not particularly care about
// display format related fields, or intends to set them later. It makes it so
// the generic type parameter of the Criterion can be inferred from the function
// given, which cannot be done when directly instantiating Criterion as of Go
// 1.19.
//
// Instead of having to write something long and difficult to read such as
// Criterion[time.Time]{Meets: func(v time.Time){ return v == myTime}}, this
// function can be used to write Meets(func(v time.Time){ return v == myTime}),
// which is a bit easier to grok.
//
// baseName, if given, gives the baseName to use for the display field. Only the
// first baseName is read, if present; all after the first are ignored. If one
// is given, it is used as the basis for both Format and NotFormat. If one is
// provided and it is blank, this function will panic.
//
// In order to avoid breaking the contract that functions with the same format
// strings return equivalent values from their Meets methods, and given that fn
// itself cannot be exhaustively checked to ensure that, every Criterion created
// with Meets that doesn't provide a baseName is given a random name that is
// used to fill its display-format related fields. The randomness is not suited
// for cryptographic applications. If this is needed, callers should avoid using
// the default name generation by providing a base-name themselves.
func Meets[E any](fn func(v E) bool, baseName ...string) Criterion[E] {
	var funcName string

	if len(baseName) > 0 {
		if baseName[0] == "" {
			panic("Meets() called with explicitly empty baseName")
		}

		funcName = baseName[0]
	} else {
		var typeParamInst E
		typeName := fmt.Sprintf("%T", typeParamInst)

		// gen 120 random bits for the name (15 bytes). This will give 20 base64
		// characters with no padding, nearly as unlikely to collide as UUIDs.
		randBits := make([]byte, 15)
		rng.Read(randBits)
		randStr := base64.StdEncoding.EncodeToString(randBits)

		funcName = fmt.Sprintf("CHECK_%s_%s", strings.ToUpper(typeName), randStr)
	}

	return Criterion[E]{
		Meets:     fn,
		Format:    funcName + "(%s)",
		NotFormat: "!" + funcName + "(%s)",
	}
}
