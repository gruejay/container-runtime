#!/bin/sh

echo "Current PID: $$"
echo "Parent PID: $PPID"
echo ""
echo "Process running as PID 1:"
ps -p 1 -o pid,ppid,comm,args
echo ""
echo "All processes:"
ps aux
echo ""
echo "PID 1 command line:"
cat /proc/1/cmdline | tr '\0' ' ' > pid.txt
echo ""
