#!/bin/bash
set -e

export hosts="
172.17.100.103
172.17.100.104
172.17.100.106
172.17.100.110
172.17.200.51
172.17.200.52
172.17.200.53
172.17.200.54
172.17.200.55
"

for host in $hosts; do
    # scp ./monclient root@$host:/usr/local/bin/
    ssh root@$host "nohup monclient > /dev/null 2>&1 &"
done