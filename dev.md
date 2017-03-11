## Dev Notes

It would be nice to unify all the message id stuff. Move ID out of query and
response and into wrap, then have it set to 0 when sent over the wire but use
it as the message for the packets.