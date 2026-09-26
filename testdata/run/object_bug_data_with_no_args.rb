# skip: Data#with with no arguments returns a copy; MRI returns the receiver itself, so equal? is true

# rbs_inline: enabled

Coord = Data.define(:lat, :lng) #: [Float, Float]

here = Coord.new(lat: 1.0, lng: 2.0)
puts here.with.equal?(here).inspect, here.with(lat: 1.0).equal?(here).inspect, (here.with == here).inspect
