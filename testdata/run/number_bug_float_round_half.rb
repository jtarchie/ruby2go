# skip: Float#round uses math.RoundToEven; MRI rounds halves away from zero (2.5.round is 3, rb2go 2)
# rbs_inline: enabled

puts 2.5.round.inspect, 0.5.round.inspect, -2.5.round.inspect, -0.5.round.inspect, 1.5.round.inspect, 3.5.round.inspect
