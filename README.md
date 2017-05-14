```
usage: eye-of-charles [<flags>] <field> <object>

A simple computer vision tool for finding objests amongst fields. Outputs a list of hits
(X, Y, and pixel values) that are above or below the provided threshholds.

Flags:
      --help     Show context-sensitive help (also try --help-long and --help-man).
      --low=0    output hits below this thresshold.
      --high=1   output hits above this threshhold.
      --dist=-1  minimum pixel distance between hits. if negative, use object image size.
  -v, --verbose  verbose output
      --png      enable png output

Args:
  <field>   field (game screen) image
  <object>  object to find
```
