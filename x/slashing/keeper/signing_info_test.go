package keeper_test

import (
	"time"

	"cosmossdk.io/core/comet"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/slashing/testutil"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"go.uber.org/mock/gomock"
)

func (s *KeeperTestSuite) TestValidatorSigningInfo() {
	ctx, keeper := s.ctx, s.slashingKeeper
	require := s.Require()

	signingInfo := slashingtypes.NewValidatorSigningInfo(
		consAddr,
		ctx.BlockHeight(),
		int64(3),
		time.Unix(2, 0),
		false,
		int64(10),
	)

	// set the validator signing information
	require.NoError(keeper.SetValidatorSigningInfo(ctx, consAddr, signingInfo))

	require.True(keeper.HasValidatorSigningInfo(ctx, consAddr))
	info, err := keeper.GetValidatorSigningInfo(ctx, consAddr)
	require.NoError(err)
	require.Equal(info.StartHeight, ctx.BlockHeight())
	require.Equal(info.IndexOffset, int64(3))
	require.Equal(info.JailedUntil, time.Unix(2, 0).UTC())
	require.Equal(info.MissedBlocksCounter, int64(10))

	var signingInfos []slashingtypes.ValidatorSigningInfo

	require.NoError(keeper.IterateValidatorSigningInfos(ctx, func(consAddr sdk.ConsAddress, info slashingtypes.ValidatorSigningInfo) (stop bool) {
		signingInfos = append(signingInfos, info)
		return false
	}))

	require.Equal(signingInfos[0].Address, signingInfo.Address)

	// test Tombstone
	err = keeper.Tombstone(ctx, consAddr)
	require.NoError(err)
	require.True(keeper.IsTombstoned(ctx, consAddr))

	// test JailUntil
	jailTime := time.Now().Add(time.Hour).UTC()
	require.NoError(keeper.JailUntil(ctx, consAddr, jailTime))
	sInfo, _ := keeper.GetValidatorSigningInfo(ctx, consAddr)
	require.Equal(sInfo.JailedUntil, jailTime)
}

func (s *KeeperTestSuite) TestValidatorMissedBlockBitmap_SmallWindow() {
	ctx, keeper := s.ctx, s.slashingKeeper
	require := s.Require()

	for _, window := range []int64{100, 32_000} {
		params := testutil.TestParams()
		params.SignedBlocksWindow = window
		require.NoError(keeper.SetParams(ctx, params))

		// validator misses all blocks in the window
		var valIdxOffset int64
		for valIdxOffset < params.SignedBlocksWindow {
			idx := valIdxOffset % params.SignedBlocksWindow
			err := keeper.SetMissedBlockBitmapValue(ctx, consAddr, idx, true)
			require.NoError(err)

			missed, err := keeper.GetMissedBlockBitmapValue(ctx, consAddr, idx)
			require.NoError(err)
			require.True(missed)

			valIdxOffset++
		}

		// validator should have missed all blocks
		missedBlocks, err := keeper.GetValidatorMissedBlocks(ctx, consAddr)
		require.NoError(err)
		require.Len(missedBlocks, int(params.SignedBlocksWindow))

		// sign next block, which rolls the missed block bitmap
		idx := valIdxOffset % params.SignedBlocksWindow
		err = keeper.SetMissedBlockBitmapValue(ctx, consAddr, idx, false)
		require.NoError(err)

		missed, err := keeper.GetMissedBlockBitmapValue(ctx, consAddr, idx)
		require.NoError(err)
		require.False(missed)

		// validator should have missed all blocks except the last one
		missedBlocks, err = keeper.GetValidatorMissedBlocks(ctx, consAddr)
		require.NoError(err)
		require.Len(missedBlocks, int(params.SignedBlocksWindow)-1)
	}
}

func (s *KeeperTestSuite) TestHandleValidatorSignatureSkipsHealthySignedBlock() {
	ctx, keeper := s.ctx.WithBlockHeight(10), s.slashingKeeper
	require := s.Require()

	signingInfo := slashingtypes.NewValidatorSigningInfo(
		consAddr,
		1,
		0,
		time.Unix(0, 0).UTC(),
		false,
		0,
	)
	require.NoError(keeper.SetValidatorSigningInfo(ctx, consAddr, signingInfo))

	s.stakingKeeper.EXPECT().IsValidatorJailed(gomock.Any(), consAddr).Return(false, nil)

	require.NoError(keeper.HandleValidatorSignature(ctx, consAddr.Bytes(), 1, comet.BlockIDFlagCommit))

	info, err := keeper.GetValidatorSigningInfo(ctx, consAddr)
	require.NoError(err)
	require.Equal(int64(0), info.IndexOffset)
	require.Equal(int64(0), info.MissedBlocksCounter)
}

func (s *KeeperTestSuite) TestHandleValidatorSignatureSkipsSignedBlockWithoutBitmapChange() {
	ctx, keeper := s.ctx.WithBlockHeight(10), s.slashingKeeper
	require := s.Require()

	signingInfo := slashingtypes.NewValidatorSigningInfo(
		consAddr,
		1,
		42,
		time.Unix(0, 0).UTC(),
		false,
		1,
	)
	require.NoError(keeper.SetValidatorSigningInfo(ctx, consAddr, signingInfo))
	require.NoError(keeper.SetMissedBlockBitmapValue(ctx, consAddr, 0, true))

	s.stakingKeeper.EXPECT().IsValidatorJailed(gomock.Any(), consAddr).Return(false, nil)

	require.NoError(keeper.HandleValidatorSignature(ctx, consAddr.Bytes(), 1, comet.BlockIDFlagCommit))

	info, err := keeper.GetValidatorSigningInfo(ctx, consAddr)
	require.NoError(err)
	require.Equal(int64(42), info.IndexOffset)
	require.Equal(int64(1), info.MissedBlocksCounter)
}

func (s *KeeperTestSuite) TestHandleValidatorSignatureUpdatesRecoveredMissedBlock() {
	ctx, keeper := s.ctx.WithBlockHeight(10), s.slashingKeeper
	require := s.Require()

	signingInfo := slashingtypes.NewValidatorSigningInfo(
		consAddr,
		1,
		42,
		time.Unix(0, 0).UTC(),
		false,
		1,
	)
	require.NoError(keeper.SetValidatorSigningInfo(ctx, consAddr, signingInfo))
	require.NoError(keeper.SetMissedBlockBitmapValue(ctx, consAddr, 9, true))

	s.stakingKeeper.EXPECT().IsValidatorJailed(gomock.Any(), consAddr).Return(false, nil)

	require.NoError(keeper.HandleValidatorSignature(ctx, consAddr.Bytes(), 1, comet.BlockIDFlagCommit))

	info, err := keeper.GetValidatorSigningInfo(ctx, consAddr)
	require.NoError(err)
	require.Equal(int64(42), info.IndexOffset)
	require.Equal(int64(0), info.MissedBlocksCounter)

	missed, err := keeper.GetMissedBlockBitmapValue(ctx, consAddr, 9)
	require.NoError(err)
	require.False(missed)
}
