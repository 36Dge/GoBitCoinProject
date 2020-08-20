package blockchain

//timesorter implements sort.interface to allow a slice of timestamps to
//be sorted
type timeSorter []int64

//len returns the number of timestamps in the slice .it is part of the sort.interface
//implementation.
func (s timeSorter)Len()int  {
	return len(s)
}

//swap the timestamps at the passed indices .it is part of the
//sort.interface implementation.
func (s timeSorter)Swap(i,j int)  {
	s[i],s[j] = s[j],s[i]
}

//less returns whether the timestamp with index i should
func (s timeSorter)Less(i,j int)bool  {
	return s[i] < s[j]
}
//over
