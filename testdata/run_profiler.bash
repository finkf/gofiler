#!/bin/bash

while read line; do
	echo "$line" >&2
done
cat testdata/profile.json
