# skip: Array#[] of a String hands out a pointer into the Array's storage, so a later write to the slot shows through (#138; decision 397 keeps it: the copy cost spinel's bm_csv_process +5%, rbOpt[String] is the fix)

a = ["x"]
s = a[-1]
a[-1] = "y"
p s

m = %w[p q]
x1, x2 = m
m[0] = "z"
p x1, x2
