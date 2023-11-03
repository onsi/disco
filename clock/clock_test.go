package clock_test

import (
	"time"

	"github.com/onsi/disco/clock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Clock", func() {
	DescribeTable("NextSaturday", func(input, output time.Time) {
		Expect(clock.NextSaturdayAt10(input)).To(Equal(output))
	},
		Entry("when it's before Saturday at 10",
			time.Date(2023, time.September, 26, 13, 07, 35, 0, clock.Timezone),
			time.Date(2023, time.September, 30, 10, 0, 0, 0, clock.Timezone),
		),
		Entry("when it's just before Saturday at 10",
			time.Date(2023, time.September, 30, 9, 59, 59, 0, clock.Timezone),
			time.Date(2023, time.September, 30, 10, 0, 0, 0, clock.Timezone),
		),
		Entry("when it's just after Saturday at 10",
			time.Date(2023, time.September, 30, 10, 0, 01, 0, clock.Timezone),
			time.Date(2023, time.October, 7, 10, 0, 0, 0, clock.Timezone),
		),
		Entry("no problem at leap years",
			time.Date(2024, time.February, 28, 12, 0, 0, 0, clock.Timezone),
			time.Date(2024, time.March, 2, 10, 0, 0, 0, clock.Timezone),
		),
	)

	DescribeTable("NextSaturdayAt10Or1030", func(input, output time.Time) {
		Expect(clock.NextSaturdayAt10Or1030(input)).To(Equal(output))
	},
		Entry("when it's before Saturday at 10 (during DST)",
			time.Date(2023, time.October, 30, 13, 07, 35, 0, clock.Timezone),
			time.Date(2023, time.November, 4, 10, 0, 0, 0, clock.Timezone),
		),
		Entry("when it's just before Saturday at 10 (during DST)",
			time.Date(2023, time.November, 4, 9, 59, 59, 0, clock.Timezone),
			time.Date(2023, time.November, 4, 10, 0, 0, 0, clock.Timezone),
		),
		Entry("when it's just after Saturday at 10 (during DST)",
			time.Date(2023, time.October, 28, 10, 0, 01, 0, clock.Timezone),
			time.Date(2023, time.November, 4, 10, 0, 0, 0, clock.Timezone),
		),
		Entry("when it's before Saturday at 10:30 (if not DST)",
			time.Date(2023, time.November, 6, 7, 07, 35, 0, clock.Timezone),
			time.Date(2023, time.November, 11, 10, 30, 0, 0, clock.Timezone),
		),
		Entry("when it's just before Saturday at 10:30 (druing DST)",
			time.Date(2023, time.November, 11, 10, 29, 29, 0, clock.Timezone),
			time.Date(2023, time.November, 11, 10, 30, 0, 0, clock.Timezone),
		),
		Entry("when it's just after Saturday at 10:30 (during DST)",
			time.Date(2023, time.November, 11, 10, 30, 01, 0, clock.Timezone),
			time.Date(2023, time.November, 18, 10, 30, 0, 0, clock.Timezone),
		),
		Entry("no problem at leap years",
			time.Date(2024, time.February, 28, 12, 0, 0, 0, clock.Timezone),
			time.Date(2024, time.March, 2, 10, 30, 0, 0, clock.Timezone),
		),
		Entry("when we're starting DST",
			time.Date(2024, time.March, 9, 10, 30, 0, 0, clock.Timezone),
			time.Date(2024, time.March, 16, 10, 0, 0, 0, clock.Timezone),
		),
	)

	Describe("AlarmClock", func() {
		var c *clock.AlarmClock

		BeforeEach(func() {
			c = clock.NewAlarmClock()
			DeferCleanup(c.Stop)
		})

		It("fires an alarm at the specified time", func() {
			c.SetAlarm(time.Now().Add(time.Millisecond * 100))
			Eventually(c.C()).WithTimeout(time.Millisecond * 200).Should(Receive())
			Consistently(c.C()).WithTimeout(time.Millisecond * 200).ShouldNot(Receive())
		})

		It("resets the alarm when called again", func() {
			c.SetAlarm(time.Now().Add(time.Millisecond * 400))
			c.SetAlarm(time.Now().Add(time.Millisecond * 100))
			Eventually(c.C()).WithTimeout(time.Millisecond * 200).Should(Receive())
			Consistently(c.C()).WithTimeout(time.Millisecond * 300).ShouldNot(Receive())
		})

		It("fires basically immediately when given a time in the past", func() {
			c.SetAlarm(time.Now().Add(-time.Millisecond))
			Eventually(c.C()).WithTimeout(time.Millisecond * 100).Should(Receive())
		})

		It("stops sending on the channel when stopped, even if an alarm is already going off", func() {
			c.SetAlarm(time.Now().Add(time.Millisecond * 50))
			time.Sleep(200 * time.Millisecond)
			c.Stop()
			time.Sleep(100 * time.Millisecond)
			Consistently(c.C()).WithTimeout(time.Millisecond * 100).ShouldNot(Receive())
			c.SetAlarm(time.Now().Add(time.Millisecond * 100))
			Eventually(c.C()).WithTimeout(time.Millisecond * 200).Should(Receive())

		})

		It("can be reused after stop", func() {
			c.SetAlarm(time.Now().Add(time.Millisecond * 400))
			c.Stop()
			c.SetAlarm(time.Now().Add(time.Millisecond * 100))
			Eventually(c.C()).WithTimeout(time.Millisecond * 200).Should(Receive())
		})
	})
})
