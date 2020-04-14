package builtin.check.update

# Default check for updating Kubernetes object updates.

# Assert that the  UID and resource version matches.
default input_is_last_applied = false
input_is_last_applied {
    latest := input.latest
    last := data.resources.applied.last

    latest.metadata.uid == last.metadata.uid
    latest.metadata.resourceVersion == last.metadata.resourceVersion
}

fatal_input_is_not_latest[msg] {
  not input.error.message
  not input_is_last_applied

  msg := "input.latest is not the same object as data.resources.applied.last"
}

fatal_input_is_not_latest[msg] {
  not input.error.message
  not input_is_last_applied

  latest := input.latest
  last := data.resources.applied.last

  msg := [
    sprintf("latest UID is %q, last UID is %q", [
        latest.metadata.uid, last.metadata.uid]),
    sprintf("latest resourceVersion is %q, last UID resourceVersion %q", [
        latest.metadata.resourceVersion, last.metadata.resourceVersion]),
  ]
}

fatal_update_error[msg] {
  # If the Error field is present, the update failed.
  input.error.message

  msg := sprintf("failed to update %s '%s/%s': %s", [
    input.target.meta.kind,
    input.target.namespace,
    input.target.name,
    input.error.message,
  ])
}

# vim: ts=2 sts=2 sw=2 et:
