package warrior

import (
	"time"

	"github.com/wowsims/wotlk/sim/core"
	"github.com/wowsims/wotlk/sim/core/proto"
	"github.com/wowsims/wotlk/sim/core/stats"
)

func (warrior *Warrior) registerShieldSlamSpell() {
	cost := 20.0 - float64(warrior.Talents.FocusedRage)
	refundAmount := cost * 0.8

	hasGlyph := warrior.HasMajorGlyph(proto.WarriorMajorGlyph_GlyphOfBlocking)
	var glyphOfBlockingAura *core.Aura = nil
	if hasGlyph {
		statDep := warrior.NewDynamicMultiplyStat(stats.BlockValue, 1.1)
		glyphOfBlockingAura = warrior.GetOrRegisterAura(core.Aura{
			Label:    "Glyph of Blocking",
			ActionID: core.ActionID{SpellID: 58397},
			Duration: 10 * time.Second,
			OnGain: func(aura *core.Aura, sim *core.Simulation) {
				aura.Unit.EnableDynamicStatDep(sim, statDep)
			},
			OnExpire: func(aura *core.Aura, sim *core.Simulation) {
				aura.Unit.DisableDynamicStatDep(sim, statDep)
			},
		})
	}

	damageRollFunc := core.DamageRollFunc(990, 1040)

	warrior.ShieldSlam = warrior.RegisterSpell(core.SpellConfig{
		ActionID:    core.ActionID{SpellID: 47488},
		SpellSchool: core.SpellSchoolPhysical,
		Flags:       core.SpellFlagMeleeMetrics,

		ResourceType: stats.Rage,
		BaseCost:     cost,

		Cast: core.CastConfig{
			DefaultCast: core.Cast{
				Cost: cost,
				GCD:  core.GCDDefault,
			},
			ModifyCast: func(sim *core.Simulation, spell *core.Spell, cast *core.Cast) {
				if warrior.SwordAndBoardAura.IsActive() {
					cast.Cost = 0

					warrior.SwordAndBoardAura.Deactivate(sim)
				}
			},
			IgnoreHaste: true,
			CD: core.Cooldown{
				Timer:    warrior.NewTimer(),
				Duration: time.Second * 6,
			},
		},

		ApplyEffects: core.ApplyEffectFuncDirectDamage(core.SpellEffect{
			ProcMask: core.ProcMaskMeleeMHSpecial, // TODO: Is this right?

			BonusCritRating:  5 * core.CritRatingPerCritChance * float64(warrior.Talents.CriticalBlock),
			DamageMultiplier: (1.0 + 0.05*float64(warrior.Talents.GagOrder)) * core.TernaryFloat64(warrior.HasSetBonus(ItemSetOnslaughtArmor, 4), 1.1, 1), // TODO: GagOrder might apply differently
			ThreatMultiplier: 1.3,
			FlatThreatBonus:  770,

			BaseDamage: core.BaseDamageConfig{
				Calculator: func(sim *core.Simulation, _ *core.SpellEffect, _ *core.Spell) float64 {
					sbv := warrior.GetStat(stats.BlockValue)
					sbvBonus := core.TernaryFloat64(sbv <= 1960.0, sbv, 0.0) + core.TernaryFloat64(sbv > 1960.0 && sbv <= 3160.0, 0.09333333333*sbv+1777.06666667, 0.0) + core.TernaryFloat64(sbv > 3160.0, 2072.0, 0.0)
					return damageRollFunc(sim) + sbvBonus
				},
				TargetSpellCoefficient: 1,
			},
			OutcomeApplier: warrior.OutcomeFuncMeleeSpecialHitAndCrit(warrior.critMultiplier(mh)),

			OnSpellHitDealt: func(sim *core.Simulation, spell *core.Spell, spellEffect *core.SpellEffect) {
				if !spellEffect.Landed() {
					warrior.AddRage(sim, refundAmount, warrior.RageRefundMetrics)
				} else {
					if glyphOfBlockingAura != nil {
						glyphOfBlockingAura.Activate(sim)
					}
				}
			},
		}),
	})
}

func (warrior *Warrior) HasEnoughRageForShieldSlam() bool {
	if warrior.SwordAndBoardAura != nil {
		if warrior.SwordAndBoardAura.IsActive() {
			return true
		}
	}

	return warrior.CurrentRage() >= warrior.ShieldSlam.DefaultCast.Cost
}

func (warrior *Warrior) CanShieldSlam(sim *core.Simulation) bool {
	return warrior.PseudoStats.CanBlock && warrior.HasEnoughRageForShieldSlam() && warrior.ShieldSlam.IsReady(sim)
}
