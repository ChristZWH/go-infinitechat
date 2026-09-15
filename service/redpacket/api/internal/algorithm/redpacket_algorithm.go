package algorithm

import (
	"fmt"
	"math/rand"
	"sort"
)

const MinAmount int64 = 1

// AllocateNormalRedPacket 普通红包（均分）
func AllocateNormalRedPacket(totalAmount int64, totalCount int) ([]int64, error) {
	if totalAmount <= 0 || totalCount <= 0 {
		return nil, fmt.Errorf("红包金额和数量必须大于0")
	}
	if totalAmount < int64(totalCount)*MinAmount {
		return nil, fmt.Errorf("红包总金额不足，至少需要%d分", int64(totalCount)*MinAmount)
	}

	amounts := make([]int64, totalCount)
	avgAmount := totalAmount / int64(totalCount)
	remainder := totalAmount % int64(totalCount)

	for i := 0; i < totalCount; i++ {
		amounts[i] = avgAmount
	}
	for i := 0; i < int(remainder); i++ {
		amounts[i]++
	}
	return amounts, nil
}

// 拼手气红包（预处理线段切割法）
func AllocateRandomRedPacket(totalAmount int64, totalCount int) ([]int64, error) {
	if totalAmount <= 0 || totalCount <= 0 {
		return nil, fmt.Errorf("红包金额和数量必须大于0")
	}
	if totalAmount < int64(totalCount)*MinAmount {
		return nil, fmt.Errorf("红包总金额不足，至少需要%d分", int64(totalCount)*MinAmount)
	}
	if totalCount == 1 {
		return []int64{totalAmount}, nil
	}

	remainAmount := totalAmount - int64(totalCount)*MinAmount
	// 刚好满足最低金额，无需分配，直接返回
	if remainAmount == 0 {
		amount := make([]int64, 0, totalCount)
		for item := range amount {
			amount[item] = MinAmount
		}
		return amount, nil
	}

	// 切点法需要 totalCount+1 个互不相同的整数点，而区间 [0, remainAmount]
	// 只有 remainAmount+1 个整数；remainAmount < totalCount 时点数不够，
	// 循环永远凑不齐（死循环），改用"随机挑人加 1 分"分配。
	// 注意这里也覆盖了 remainAmount == 0（每人恰好保底）的情况。
	if remainAmount < int64(totalCount) {
		amounts := make([]int64, totalCount)
		for i := range amounts {
			amounts[i] = MinAmount
		}
		for i := int64(0); i < remainAmount; i++ {
			amounts[rand.Intn(totalCount)]++
		}
		return amounts, nil
	}

	cutSet := make(map[int64]bool)
	cutSet[0] = true            // 左端点
	cutSet[remainAmount] = true // 右端点

	// 这里不是 <=，等于会导致多一个值
	for len(cutSet) < totalCount+1 {
		cutSet[rand.Int63n(remainAmount+1)] = true
	}

	sortedPoints := make([]int64, 0, len(cutSet))
	for p := range cutSet {
		sortedPoints = append(sortedPoints, p)
	}
	sort.Slice(sortedPoints, func(i, j int) bool {
		return sortedPoints[i] < sortedPoints[j]
	})
	amounts := make([]int64, totalCount)
	for i := 0; i < totalCount; i++ {
		amounts[i] = sortedPoints[i+1] - sortedPoints[i] + MinAmount
	}

	rand.Shuffle(len(amounts), func(i, j int) {
		amounts[i], amounts[j] = amounts[j], amounts[i]
	})

	var sum int64
	for _, a := range amounts {
		sum += a
	}
	if sum != totalAmount {
		amounts[len(amounts)-1] += totalAmount - sum
	}
	return amounts, nil
}
