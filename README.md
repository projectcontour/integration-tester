# Integration Tester for Kubernetes

Integration Tester for Kubernetes is a test driver that helps run
integration tests for Kubernetes controllers.

# Writing tests

Test documents are strucured as a sequence of YAML and Rego document
separated by the YAML document separator, `---`.

## Fixtures

The `run` command takes a `--fixtures` flag. This flag can be used
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

## Skipping tests

If there is a skip rule (any rule whose name begins with the string
"skip"), `integration-tester` will evaluate is for any results. If the
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
