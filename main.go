package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rickbassham/fitsrename/common"
	"github.com/rickbassham/fitsrename/fits"
	"github.com/rickbassham/fitsrename/xisf"
	"github.com/yargevad/filepathx"
)

type defaultMap map[string]string

func (d defaultMap) String() string {
	if d == nil {
		return ""
	}

	var result strings.Builder

	for k, v := range d {
		result.WriteString(fmt.Sprintf("%s=%s;", k, v))
	}

	return result.String()
}

func (d defaultMap) Set(s string) error {
	pairs := strings.Split(s, ";")
	for i := range pairs {
		kv := strings.Split(pairs[i], "=")
		if len(kv) != 2 {
			return errors.New("invalid defaults value")
		}

		d[kv[0]] = kv[1]
	}

	return nil
}

var (
	input = flag.String("input", "*.fits", "Glob to match files.")

	debug   = flag.Bool("debug", false, "Enable debug logging")
	noSpace = flag.Bool("no-space", false, "Replace spaces in tokens with underscore.")

	light = flag.String("light", "", "Format to rename lights to. In the form of {FITSKEYWORD1}_{FITSKEYWORD2:%0.2f}.fits")
	dark  = flag.String("dark", "", "Format to rename darks to. In the form of {FITSKEYWORD1}_{FITSKEYWORD2:%0.2f}.fits")
	flat  = flag.String("flat", "", "Format to rename flats to. In the form of {FITSKEYWORD1}_{FITSKEYWORD2:%0.2f}.fits")
	bias  = flag.String("bias", "", "Format to rename biases to. In the form of {FITSKEYWORD1}_{FITSKEYWORD2:%0.2f}.fits")

	suffix = flag.String("suffix", "%03d", "What to append to the end of the file. It will be sent to fmt.Sprintf with the file number for the current directory.")

	dryRun    = flag.Bool("dry-run", false, "Don't actually rename the files, just print what we would do.")
	defaults  = defaultMap{}
	overrides = defaultMap{}

	ignoreWarnings = flag.Bool("ignore-warnings", false, "Ignore checks that protect you from deleting data. Dangerous.")

	aliases = map[string][]string{
		"EXPTIME": []string{"EXPOSURE"},
	}

	checkSuffix  = regexp.MustCompile(`%\d*d`)
	formatSuffix = true

	//parserRegex = regexp.MustCompile(`\{([^:]+?)(:(.*?))?\}`)
)

type token struct {
	raw    bool
	header string
	format string
}

func convertInt(n interface{}) int64 {
	switch n := n.(type) {
	case int:
		return int64(n)
	case int8:
		return int64(n)
	case int16:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return int64(n)
	}

	log.Fatalln(fmt.Sprintf("%T is not an int", n))
	return 0
}

func convertUInt(n interface{}) uint64 {
	switch n := n.(type) {
	case uint:
		return uint64(n)
	case uint8:
		return uint64(n)
	case uint16:
		return uint64(n)
	case uint32:
		return uint64(n)
	case uint64:
		return uint64(n)
	}

	log.Fatalln(fmt.Sprintf("%T is not an uint", n))
	return 0
}

func (t token) convert(hdr common.Header) string {
	if t.raw {
		return t.header
	}

	val, ok := hdr[t.header]
	if !ok || val == nil {
		val = defaults[t.header]
	}

	if or, ok := overrides[t.header]; ok {
		val = or
	}

	if val == nil || val == "" {
		if as, ok := aliases[t.header]; ok {
			for _, a := range as {
				val, ok := hdr[a]
				if !ok || val == nil {
					val = defaults[a]
				}

				if val != nil {
					break
				}
			}
		}

		if val == nil || val == "" {
			log.Fatalln(fmt.Sprintf("file is missing fits header %s", t.header))
		}
	}

	switch val := val.(type) {
	case string:
		if strings.HasPrefix(t.format, "date") {
			parsed, err := time.ParseInLocation("2006-01-02T15:04:05.999999999Z", val, time.UTC)
			if err != nil {
				parsed, err = time.ParseInLocation("2006-01-02T15:04:05.999999999", val, time.UTC)

				if err != nil {
					log.Fatalln(fmt.Sprintf("unable to parse %s as a timestamp", val))
				}
			}

			if t.format == "dateunix" {
				return fmt.Sprintf("%d", parsed.Unix())
			}

			return parsed.Format(t.format[4:])
		}

		return strings.TrimSpace(val)

	case bool:
		if t.format == "" {
			return fmt.Sprintf("%t", val)
		}
		return fmt.Sprintf(t.format, val)

	case int, int8, int16, int32, int64:
		v := convertInt(val)

		if t.format == "" {
			return fmt.Sprintf("%d", v)
		}

		if strings.Index(t.format, "f") > 0 {
			return fmt.Sprintf(t.format, float64(v))
		}

		return fmt.Sprintf(t.format, val)

	case uint, uint8, uint16, uint32, uint64:
		v := convertUInt(val)

		if t.format == "" {
			return fmt.Sprintf("%d", v)
		}

		if strings.Index(t.format, "f") > 0 {
			return fmt.Sprintf(t.format, float64(v))
		}

		return fmt.Sprintf(t.format, val)

	case float32:
		if t.format == "" {
			return fmt.Sprintf("%f", val)
		}

		if strings.Index(t.format, "d") > 0 {
			return fmt.Sprintf(t.format, int64(val))
		}

		return fmt.Sprintf(t.format, val)

	case float64:
		if t.format == "" {
			return fmt.Sprintf("%f", val)
		}

		if strings.Index(t.format, "d") > 0 {
			return fmt.Sprintf(t.format, int64(val))
		}

		return fmt.Sprintf(t.format, val)

	default:
		log.Fatalln(fmt.Sprintf("uknown type %T for fits header %s", val, t.header))
	}

	return ""
}

func scanForTokens(data string) []token {
	var tokens []token

	for i := 0; i < len(data); i++ {
		nextTokenStart := strings.Index(data[i:], "{")
		nextTokenEnd := strings.Index(data[i:], "}")

		if nextTokenStart == 0 && nextTokenEnd > 0 {
			// we are inside a {} token, let's look for a format specifier
			formatIndex := strings.Index(data[i+nextTokenStart:i+nextTokenEnd], ":")
			if formatIndex > 0 {
				tokens = append(tokens, token{
					raw:    false,
					header: data[i+nextTokenStart+1 : i+formatIndex],
					format: data[i+formatIndex+1 : i+nextTokenEnd],
				})
			} else {
				tokens = append(tokens, token{raw: false, header: data[i+nextTokenStart+1 : i+nextTokenEnd]})
			}
			i += nextTokenEnd
		} else if nextTokenStart > 0 {
			tokens = append(tokens, token{raw: true, header: data[i : i+nextTokenStart]})
			i += nextTokenStart - 1
		} else {
			tokens = append(tokens, token{raw: true, header: data[i:]})
			break
		}
	}

	return tokens
}

func getFileNumberPath(path string) string {
	for i := 0; ; i++ {
		testPath := path + fmt.Sprintf(*suffix, i)

		if _, err := os.Stat(testPath); os.IsNotExist(err) {
			return testPath
		}
	}
}

func handleFile(tokensByType map[string][]token, file string) {
	if *debug {
		log.Println(fmt.Sprintf("processing file %s", file))
	}

	if strings.HasPrefix(filepath.Base(file), ".") {
		log.Println(fmt.Sprintf("skipping dotfile %s", file))
		return
	}

	f, err := os.Open(file)
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer f.Close()

	var hdr common.Header

	if strings.HasSuffix(file, ".xisf") {
		hdr, err = xisf.NewDecoder(f).ReadHeader()
		if err != nil {
			log.Fatalln(err.Error())
		}
	} else if strings.HasSuffix(file, ".fits") {
		hdr, err = fits.NewDecoder(f).ReadHeader()
		if err != nil {
			log.Fatalln(err.Error())
		}
	}

	if *debug {
		for key, v := range hdr {
			switch val := v.(type) {
			case string:
				println(fmt.Sprintf("%-10s %s", key, val))
			case bool:
				println(fmt.Sprintf("%-10s %t", key, val))
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
				println(fmt.Sprintf("%-10s %d", key, val))
			case float32, float64:
				println(fmt.Sprintf("%-10s %f", key, val))
			default:
				log.Println(fmt.Sprintf("%T", val))
				panic(fmt.Sprintf("%T", val))
			}
		}
	}

	var ok bool

	t, ok := hdr["IMAGETYP"]
	if !ok || t == nil {
		t, ok = hdr["FRAME"]

		if !ok {
			log.Println(fmt.Sprintf("missing IMAGETYP and FRAME header; skipping file %s", file))
			return
		}
	}

	if t == nil {
		log.Println(fmt.Sprintf("missing IMAGETYP and FRAME header; skipping file %s", file))
		return
	}

	var imgType string
	if imgType, ok = t.(string); !ok {
		log.Println(fmt.Sprintf("IMAGETYP or FRAME header is not a string value; skipping file %s", file))
		return
	}

	var tokens []token
	switch imgType {
	case "Light Frame", "Tricolor Image", "Light", "LIGHT":
		tokens = tokensByType["LIGHT"]
	case "Dark Frame", "Dark", "DARK":
		tokens = tokensByType["DARK"]
	case "Flat Frame", "Flat", "FLAT":
		tokens = tokensByType["FLAT"]
	case "Bias Frame", "Bias", "BIAS":
		tokens = tokensByType["BIAS"]
	default:
		log.Println(fmt.Sprintf("unknown IMGTYPE or FRAME %s; skipping file %s", imgType, file))
		return
	}

	if tokens == nil {
		log.Println(fmt.Sprintf("format not specified for IMGTYPE '%s'", imgType))
		return
	}

	var result strings.Builder

	for _, tok := range tokens {
		item := tok.convert(hdr)
		if *noSpace {
			item = strings.ReplaceAll(item, " ", "_")
		}
		result.WriteString(item)
	}

	var newName string

	if formatSuffix {
		newName = getFileNumberPath(result.String())
	} else {
		result.WriteString(*suffix)
		newName = result.String()
	}

	log.Println(fmt.Sprintf("renaming %s to %s", file, newName))

	if !*dryRun {
		fullPath, _ := filepath.Abs(file)
		dir := path.Dir(fullPath)

		d, err := os.Stat(dir)
		if err != nil {
			log.Fatalln(fmt.Sprintf("unable to access %s; %s", file, err.Error()))
		}

		// new directory should match permissions of the original file's directory.
		perm := d.Mode() & os.ModePerm

		err = os.MkdirAll(path.Dir(newName), perm)
		if err != nil && !os.IsExist(err) {
			log.Fatalln(fmt.Sprintf("error renaming %s to %s; %s", file, newName, err.Error()))
		}

		err = os.Rename(file, newName)
		if err != nil {
			log.Fatalln(fmt.Sprintf("error renaming %s to %s; %s", file, newName, err.Error()))
		}
	}
}

func main() {
	flag.Var(defaults, "defaults", "Specifies default values to use if a FITS header is missing. Ex: FILTER=RGB;OBS=Me")
	flag.Var(overrides, "overrides", "Specifies values to override in a FITS header. Ex: FILTER=RGB;OBS=Me")

	flag.Parse()

	if *input == "" {
		flag.Usage()
		log.Fatal("See usage.")
	}

	if !checkSuffix.MatchString(*suffix) {
		if *ignoreWarnings {
			log.Println("Suffix does not contain a %d modifier, data could be lost if your file formats don't produce unique names.")
		} else {
			log.Fatalln("You must supply a %d modifier to your suffix to ensure unique file names, or use -ignore-warnings to override.")
		}

		formatSuffix = false
	}

	defer log.Println("done")

	log.Println(fmt.Sprintf("light pattern: %s", *light))
	log.Println(fmt.Sprintf("dark  pattern: %s", *dark))
	log.Println(fmt.Sprintf("flat  pattern: %s", *flat))
	log.Println(fmt.Sprintf("bias  pattern: %s", *bias))

	log.Println(fmt.Sprintf("searching for files matching %s", *input))

	files, err := filepathx.Glob(*input)
	if err != nil {
		log.Fatalln(err.Error())
	}

	log.Println(fmt.Sprintf("found %d matching files", len(files)))

	tokensByType := map[string][]token{
		"LIGHT": scanForTokens(*light),
		"DARK":  scanForTokens(*dark),
		"BIAS":  scanForTokens(*bias),
		"FLAT":  scanForTokens(*flat),
	}

	for _, file := range files {
		handleFile(tokensByType, file)
	}
}
