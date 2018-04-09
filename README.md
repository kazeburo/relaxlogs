# relaxlogs

CLI for lestrrat-go/file-rotatelogs

```
Usage:
  relaxlogs [OPTIONS]

Application Options:
      --log-dir=       Directory to store logfiles
      --rotation-time= The time between log file rotations in minutes (default: 60)
      --max-age=       Maximum age of files (based on mtime), in minutes (default: 1440)
      --with-time      Enable to prepend time
  -v, --version        Show version

Help Options:
  -h, --help           Show this help message
```

## Example: replacing multilog with relaxlogs

```
#!/bin/sh
exec /usr/local/bin/relaxlogs --log-dir /var/log/app --rotation-time 1440 --max-age 44640
```

logfile rotate in 1 day and max-age is 31 day.
logfiles become `/var/log/app/log.%Y%m%d%H%M`.
linkname becom `/var/log/app/current`.
