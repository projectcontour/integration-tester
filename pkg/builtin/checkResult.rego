package builtin.result

PassResult := "Pass"
SkipResult := "Skip"
ErrorResult := "Error"
FatalResult := "Fatal"

Pass(msg) = {
    "result": PassResult,
    "msg": msg,
}

Passf(fmt, args) = {
    "result": PassResult,
    "msg": sprintf(fmt, args),
}

Skip(msg) = {
    "result": SkipResult,
    "msg": msg,
}

Skipf(fmt, args) = {
    "result": SkipResult,
    "msg": sprintf(fmt, args),
}

Error(msg) = {
    "result": ErrorResult,
    "msg": msg,
}

Errorf(fmt, args) = {
    "result": ErrorResult,
    "msg": sprintf(fmt, args),
}

Fatal(msg) = {
    "result": FatalResult,
    "msg": msg,
}

Fatalf(fmt, args) = {
    "result": FatalResult,
    "msg": sprintf(fmt, args),
}
