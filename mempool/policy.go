package mempool

/*
At a high level, this package satisfies that requirement by
providing an in-memory pool of fully validated transactions
that can also optionally be further filtered based upon a configurable policy.
One of the policy configuration options controls whether or not "standard"
transactions are accepted. In essence, a "standard" transaction is one that
satisfies a fairly strict set of requirements that are largely intended to
help provide fair use of the system to all users. It is important to note
that what is considered a "standard" transaction changes over time.
For some insight, at the time of this writing, an example of some of
the criteria that are required for a transaction to be considered standard
are that it is of the most-recently supported version, finalized, does not
exceed a specific size, and only consists of specific script forms.

// 摘引至：Project_README.md 文件

*/



