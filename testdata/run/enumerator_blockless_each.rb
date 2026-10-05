# Nothing else reaches Array#each, so the pruner keeps only what these enumerators name (decision 49).
p [1, 2].each.next
p [1, 2].map.next
p [1, 2].select.peek
p "a\nb".each_line.next
