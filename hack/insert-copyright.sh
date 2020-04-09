#! /usr/bin/env bash

# Copyright 2020 VMware, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License.  You may obtain
# a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
# License for the specific language governing permissions and limitations
# under the License.

set -o pipefail
set -o nounset
set -o errexit

readonly YEAR=$(date +%Y)

license::go() {
    echo licensing Go file $1

    ed "$1" <<EOF
0i
// Copyright ${YEAR} VMware, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.  You may obtain
// a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
// License for the specific language governing permissions and limitations
// under the License.

.
w
q
EOF
}

license::misc() {
    echo licensing misc file $1

    ed "$1" <<EOF
0i
# Copyright ${YEAR} VMware, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License.  You may obtain
# a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
# License for the specific language governing permissions and limitations
# under the License.

.
w
q
EOF
}

for f in "$@" ; do
    case "$f" in
        *.go) license::go "$f" ;;
        *.yaml) license::misc "$f" ;;
        *.sh) license::misc "$f" ;;
        *make*) license::misc "$f" ;;
        *.rego) license::misc "$f" ;;
    esac

    shift
done
