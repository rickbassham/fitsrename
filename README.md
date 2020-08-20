# fitsrename

This utility will rename your FITS files based on their keywords. Quite useful if your
astrophotography capture program doesn't name the files quite the way you like.

## Usage

```
  -bias string
        Format to rename biases to. In the form of {FITSKEYWORD1}_{FITSKEYWORD2:%0.2f}.fits
  -dark string
        Format to rename darks to. In the form of {FITSKEYWORD1}_{FITSKEYWORD2:%0.2f}.fits
  -debug
        Enable debug logging
  -defaults value
        Specifies default values to use if a FITS header is missing. Ex: FILTER=RGB;OBS=Me
  -dry-run
        Don't actually rename the files, just print what we would do.
  -flat string
        Format to rename flats to. In the form of {FITSKEYWORD1}_{FITSKEYWORD2:%0.2f}.fits
  -ignore-warnings
        Ignore checks that protect you from deleting data. Dangerous.
  -input string
        Glob to match files. (default "*.fits")
  -light string
        Format to rename lights to. In the form of {FITSKEYWORD1}_{FITSKEYWORD2:%0.2f}.fits
  -no-space
        Replace spaces in tokens with underscore.
  -suffix string
        What to append to the end of the file. It will be sent to fmt.Sprintf with the file number for the current directory. (default "%03d")
```

## Example

```
fitsrename \
    -light "organized/{DATE-OBS:date2006-01-02}/{INSTRUME}/Light/{FILTER}/{OBJECT}/{OBJECT}_{FILTER}_{INSTRUME}_{EXPTIME:%0.2f}_gain{GAIN:%0.0f}_{CCD-TEMP:%0.2f}_{DATE-OBS:dateunix}" \
    -dark "organized/{DATE-OBS:date2006-01-02}/{INSTRUME}/Dark/{INSTRUME}_{EXPTIME:%0.2f}_gain{GAIN:%0.0f}_{CCD-TEMP:%0.2f}_{DATE-OBS:dateunix}" \
    -bias "organized/{DATE-OBS:date2006-01-02}/{INSTRUME}/Bias/{INSTRUME}_gain{GAIN:%0.0f}_{CCD-TEMP:%0.2f}_{DATE-OBS:dateunix}" \
    -flat "organized/{DATE-OBS:date2006-01-02}/{INSTRUME}/Flat/{FILTER}/{FILTER}_{INSTRUME}_gain{GAIN:%0.0f}_{DATE-OBS:dateunix}" \
    -no-space \
    -suffix ".fits" \
    -default "FILTER=RGB" \
    -input "files/**/*.fits"
```
