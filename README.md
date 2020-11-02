# Integration Tester for Kubernetes

[![Test](https://github.com/projectcontour/integration-tester/workflows/Test/badge.svg)](https://github.com/projectcontour/integration-tester/actions?query=workflow%3ATest)

Integration Tester for Kubernetes is a test driver that helps run
integration tests for Kubernetes controllers.

# Writing tests

Test documents are strucured as a sequence of YAML and Rego document
separated by the YAML document separator, `---`.

## Fixtures

The [`run`][1] command takes a `--fixtures` flag. This flag can be used
multiple times and accepts a file or directory path. In either case, it
expects all the given files to contain Kubernetes objects in YAML format.

Fixtures can be applied to test cases by naming them (by their full type
and name), and specifiying that they are fixtures:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo-server
$apply: fixture
```

In many cases, a test may need multiple instances of a fixture. To
support this, a fixture can be applied with a new name. Note that the
rename supports YAML anchors, which can be used to ensure that labels
and other fields are also updated appropriately.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo-server
$apply:
  fixture:
    as: echo-server-2
```

The fixture can be placed into a namespace by giving the new name as `namespace/name`.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo-server
$apply:
  fixture:
    as: test-namespace/echo-server-2
```

## Checking Resources

On each test run, `integration-tester` probes the Kubernetes API server
for the list of supported API resources. This is stored in the Rego data
document as `data.resources.$RESOURCE[".versions"]`. The key is named
".versions" so that it is unlikely to conflict with any legitimate
Kubernetes object name.

The contents of the ".version" key is a JSON array containing a
GroupVersionKind object for each version of the resource that the
API server supports.

You can test for specific API versions with Rego code similar to this:

```Rego

Resource := "ingresses"
Group := "extensions"
Version := "v1beta1"

default is_supported = false
is_supported {
    some n

    # Get the array of GroupVersionKind objects.
    versions := data.resources[Resource][".versions"]

    # Verify that there is some 'n' for which the versions array
    # entry matches.
    versions[n].Group == Group
    versions[n].Version == Version
}
```

## Watching Resources

`integration-tester` will label and automatically watch resources of
types that it creates. One reason that you want `integration-tester` to
track resources is so that they will be deleted at the end of a test,
unless the `--preserve` flag is given. The other is so that they are
published into the Rego store to be used by test checks.

Sometimes, resources are created as a components of higher-level
resources. Where possible, `integration-tester` causes those to be labeled
(this is what happens with pods, for example), but that is not possible
in all cases. In particular, if the high level resource has no spec
field that defines labels to be applied to the resources it generates.

One technique to work around this is to create a stub resource in the
test (e.g. and empty secret), knowing that the resource will be updated
later. `integration-tester` will label and track the resource when it
creates the stub and will update its copy when it changes.

## Writing Rego Tests

## Rego test rules

In a Rego fragment,  `integration-tester` evaluates all the rules
named `skip`, `error`, `fatal` or `check`. Other names can be used
if you prefix the rule name with one of the special result tokens,
followed by an underscore, e.g. `error_if_not_present`.

Any `skip`, `error` or `fatal` rules that evaluate to true have the
corresponding test result. `skip` results cause the remainder of
the test to be passed over, but it will not report an overall
failure. `error` results cause a specific check to fail. The test
will continue, and other errors may be detected.  `fatal` results
cause the test to fail and end immediately.

A `check` result is one that can cause a check to either pass or
fail. For example:

```Rego
import data.builtin.results

check_it_is_time[r] {
    time.now_ns() > 10
    r := results.Pass("it is time")
}
```

Checks are useful for building libraries of tests that can simply
emit results without needing to depend on the naming rules of the
top-level query. The `data.builtin.results` package contains a set
of helper functions that make constructing results easier:

| Name | Args | Description |
| -- | -- | -- |
| Pass(msg) | *string* | Construct a `pass` result with the message string. |
| Passf(msg, args) | *string*, *array* | Construct a `pass` result with a `sprintf` format string. |
| Skip(msg) | *string* | Construct a `skip` result with the message string. |
| Skipf(msg, args) | *string*, *array* | Construct a `skip` result with a `sprintf` format string. |
| Error(msg) | *string* | Construct a `error` result with the message string. |
| Errorf(msg, args) | *string*, *array* | Construct a `error` result with a `sprintf` format string. |
| Fatal(msg) | *string* | Construct a `fatal` result with the message string. |
| Fatal(msg, args) | *string*, *array* | Construct a `fatal` result with a `sprintf` format string. |

## Rego rule results

`integration-tester` supports a number of result formats for Rego
rules. The recommended format is that used by the `data.builtin.result`
module, which is a map with well-known keys `result` and `msg`:

```
{
    "result": "Pass",
    "msg": "This test passes",
}
```

`integration-tester` also supports the following result types:

* **boolean:** The rule triggers with no additional information.
* **string:** The rule triggers and the string result gives an additional reason
* **string array:** The rule triggers and the elements of the string result are joined with `\n`
* **map with `msg` key:** The rule triggers and the string result comes from the `msg` key

This Rego sample demonstrates the supported result formats:

```
error_if_true = e {
    e := true
}

error_with_message[m] {
    m = "message 1"
}

error_with_message[m] {
    m := "message 2"
}

error_with_long_message[m] {
    m := [
        "line one",
        "line 2",
    ]
}

fatal_if_message[{"msg": m}] {
    m := "fatal check"
}

```

## Skipping tests

If there is a skip rule (any rule whose name begins with the string
"skip"), `integration-tester` will evaluate it for any results. If the
skip rule has any results, the test will be skipped. This means that no
subsequent test steps will be performed, but the test as a whole will
not be considered failed.

Skip rules are also not subject to the normal check timeout, since
a condition that would cause a test to be skipped (most likely a
missing cluster feature or capability) is not likely to clear or
converge to a non-skipping state.

# References

- https://www.openpolicyagent.org/docs/latest/policy-language/
- https://www.openpolicyagent.org/docs/latest/policy-reference/
- https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/strategic-merge-patch.md

[1]: ./doc/integration-tester_run.md
