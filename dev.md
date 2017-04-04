## Dev Notes

Instead of creating a goroutine for each callback, it would be better to have
one go routine that runs as long as there are callbacks, but once the list drops
to 0, it returns. Every time we place a callback, that gets started if it's not
running.

Callback should be a struct
  Success func(r *Base)
  Error func(error)
  Timeout time.Duration