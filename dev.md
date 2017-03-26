## Dev Notes

Need a better name than ipc.Type - it identifies type, it's a base for any
simple message, it's an abstract message

I could also embed the Type in a nice wrapper. Then we wouldn't have the
unused protobuf fields. I could also hold onto a reference to the *Proc that
received the original message.

Handler auto translates to Base, but that's probably not actually good. It
should just take a message and call ToBase. I can even provide an IPC base
handle wrapper.
