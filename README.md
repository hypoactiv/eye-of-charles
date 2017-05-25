```
usage: eye-of-charles [<flags>] <field> <object>

A simple computer vision tool for finding objests amongst fields. Writes a list
of (X,Y) hits with confidence scores to out.csv if they are within the given
tolerance.

Flags:
      --help         Show context-sensitive help (also try --help-long and
                     --help-man).
  -T, --tolerance=0  only output hits with score below this tolerance.
      --dist=-1      minimum pixel distance between hits. if negative, use
                     object image size.
  -v, --verbose      verbose output
      --png          enable PNG debug output
      --no-csv       disable CSV output
      --offset=X,Y   apply offset to all hits. default: hits are centered on
                     object.
  -t, --timeout=0    timeout in milliseconds. 0 to disable timeout.
      --rect=X_MIN,Y_MIN,X_MAX,Y_MAX  
                     search for hits only within this rectangle of the field
                     image.

Args:
  <field>   field (game screen) image
  <object>  object image to find in field
```
