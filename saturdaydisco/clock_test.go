package saturdaydisco_test

import (
	"time"

	"github.com/onsi/disco/saturdaydisco"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Clock", func() {
	DescribeTable("NextSaturday", func(input, output time.Time) {
		Expect(saturdaydisco.NextSaturdayAt10(input)).To(Equal(output))
	},
		Entry("when it's before Saturday at 10",
			time.Date(2023, time.September, 26, 13, 07, 35, 0, time.Local),
			time.Date(2023, time.September, 30, 10, 0, 0, 0, time.Local),
		),
		Entry("when it's just before Saturday at 10",
			time.Date(2023, time.September, 30, 9, 59, 59, 0, time.Local),
			time.Date(2023, time.September, 30, 10, 0, 0, 0, time.Local),
		),
		Entry("when it's just after Saturday at 10",
			time.Date(2023, time.September, 30, 10, 0, 01, 0, time.Local),
			time.Date(2023, time.October, 7, 10, 0, 0, 0, time.Local),
		),
		Entry("no problem at leap years",
			time.Date(2024, time.February, 28, 12, 0, 0, 0, time.Local),
			time.Date(2024, time.March, 2, 10, 0, 0, 0, time.Local),
		),
	)

	Describe("AlarmClock", func() {
		var clock *saturdaydisco.AlarmClock

		BeforeEach(func() {
			clock = saturdaydisco.NewAlarmClock()
			DeferCleanup(clock.Stop)
		})

		It("fires an alarm at the specified time", func() {
			clock.SetAlarm(time.Now().Add(time.Millisecond * 100))
			Eventually(clock.C()).WithTimeout(time.Millisecond * 200).Should(Receive())
			Consistently(clock.C()).WithTimeout(time.Millisecond * 200).ShouldNot(Receive())
		})

		It("resets the alarm when called again", func() {
			clock.SetAlarm(time.Now().Add(time.Millisecond * 400))
			clock.SetAlarm(time.Now().Add(time.Millisecond * 100))
			Eventually(clock.C()).WithTimeout(time.Millisecond * 200).Should(Receive())
			Consistently(clock.C()).WithTimeout(time.Millisecond * 300).ShouldNot(Receive())
		})

		It("fires basically immediately when given a time in the past", func() {
			clock.SetAlarm(time.Now().Add(-time.Millisecond))
			Eventually(clock.C()).WithTimeout(time.Millisecond * 100).Should(Receive())
		})

		It("stops sending on the channel when stopped, even if an alarm is already going off", func() {
			clock.SetAlarm(time.Now().Add(time.Millisecond * 50))
			time.Sleep(200 * time.Millisecond)
			clock.Stop()
			time.Sleep(100 * time.Millisecond)
			Consistently(clock.C()).WithTimeout(time.Millisecond * 100).ShouldNot(Receive())
			clock.SetAlarm(time.Now().Add(time.Millisecond * 100))
			Eventually(clock.C()).WithTimeout(time.Millisecond * 200).Should(Receive())

		})

		It("can be reused after stop", func() {
			clock.SetAlarm(time.Now().Add(time.Millisecond * 400))
			clock.Stop()
			clock.SetAlarm(time.Now().Add(time.Millisecond * 100))
			Eventually(clock.C()).WithTimeout(time.Millisecond * 200).Should(Receive())
		})
	})
})
