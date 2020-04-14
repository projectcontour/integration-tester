package builtin.check.deletion

# Default check for deleting Kubernetes object updates.

fatal[msg] {
  # If the Error field is present, the deletion failed.
  input.error

  msg := sprintf("failed to delete %s '%s/%s': %s", [
    input.target.meta.kind,
    input.target.namespace,
    input.target.name,
    input.error.message,
  ])
}

# vim: ts=2 sts=2 sw=2 et:
